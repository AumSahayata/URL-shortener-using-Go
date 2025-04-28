package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
)

var (
	base62    = "0123456789abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ"
	validCodeRegex = regexp.MustCompile(`^[a-zA-Z0-9]+$`)
	rateLimiters = make(map[string]*rateLimiter)
	rlMutex sync.Mutex
)

type rateLimiter struct {
    lastRequest time.Time
    requests    int
}

const (
	rateLimitWindow = 1 * time.Minute
	maxRequests = 5
)

type URLData struct {
	LongURL string `json:"long_url"`
	Clicks int `json:"clicks"`
	CreatedAt int64  `json:"created_at"`
    Expiry    int64  `json:"expiry"` 
}

type Store struct {
	IDCounter int64             `json:"idCounter"`
	URLStore  map[string]URLData `json:"urlStore"`
}

func isValidCode(code string) bool {
	return validCodeRegex.MatchString(code)
}

func isValidURL(url string) bool {
	return strings.HasPrefix(url, "http://") || strings.HasPrefix(url, "https://")
}

func cleanUpExpiredLinks() {
	now := time.Now().Unix()

	urls, err := ListURLs()
	if err != nil {
		fmt.Println("Error listing URLs:", err)
        return
	}

	for _, urlData := range urls {
		code := urlData["code"].(string)
		expiresAtStr := urlData["expires_at"].(string)

		expiresAt, err1 := time.Parse(time.RFC3339, expiresAtStr)
        if err1 != nil {
            continue
        }

		if  now > expiresAt.Unix() {
			DeleteURL(code)
		}
	}

	fmt.Println("Expired links cleaned up.")
}

func encodeBase62(n int64) string {
	if n == 0 {
		return "0"
	}
	var result []byte
	for n > 0 {
		result = append([]byte{base62[n%62]}, result...)
		n /= 62
	}
	return string(result)
}

func shortenHandler(c *gin.Context) {
	var body struct {
		URL        string `json:"url"`
		CustomCode string `json:"custom_code,omitempty"`
		ExpirySeconds  int64  `json:"expiry_seconds,omitempty"`
	}

	if err := c.BindJSON(&body); err != nil || body.URL == ""{
		c.JSON(400, gin.H{"error": "Invalid request body"})
		return
	}

	if !isValidURL(body.URL) {
		c.JSON(400, gin.H{"error": "Invalid URL. Must start with http:// or https://"})
		return
	}

	var code string
	if body.CustomCode != "" {
		if !isValidCode(body.CustomCode) {
			c.JSON(400, gin.H{"error": "Invalid custom code. Use only letters and numbers"})
			return
		}
		_, err := GetURL(body.CustomCode) 
		if err == nil {
			c.JSON(409, gin.H{"error": "Custom code already in use"})
			return
		}
		code = body.CustomCode
	} else {
		id, err := GetNextID()
		if err != nil {
			c.JSON(500, gin.H{"error": "Failed to generate short code"})
			return
		}
		code = encodeBase62(id)
	}

	expiry := body.ExpirySeconds
	if expiry == 0 {
		expiry = 7 * 24 * 3600 // Default 7 days
	}

	data := URLData{
		LongURL: body.URL,
		Clicks: 0,
		CreatedAt: time.Now().Unix(),
		Expiry: expiry, // 7 days in seconds
	}

	err := SaveURL(code, data)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Error saving URL"})
		return
	}

	shortURL := fmt.Sprintf("http://localhost:8080/%s", code)
	c.JSON(200, gin.H{"short_url": shortURL})
}

func handleRedirects(c *gin.Context) {
	code := c.Param("code")

	now := time.Now().Unix()

	data, err := GetURL(code)
	if err != nil {
		c.JSON(404, gin.H{"error": "URL not found"})
		return
	}

	if data.Expiry != 0 && now > data.CreatedAt+data.Expiry {
		c.JSON(410, gin.H{"error": "URL expired"})
		return
	}
	data.Clicks++
	err = SaveURL(code, data)

	if err != nil {
		c.JSON(500, gin.H{"error": "Failed to update clicks"})
		return
	}

	c.Redirect(http.StatusFound, data.LongURL)
}

func infoHandler(c *gin.Context) {
	code := c.Param("code")
	data, err := GetURL(code)
	if err != nil {
		c.JSON(404, gin.H{"error": "Short URL not found"})
		return
	}

	current_time := time.Now().Unix()
	expiryTime := data.CreatedAt + data.Expiry

	info := gin.H{
		"long_url":   data.LongURL,
		"clicks":     data.Clicks,
		"created_at": time.Unix(data.CreatedAt, 0).UTC().Format(time.RFC3339),
		"expires_at": time.Unix(expiryTime, 0).UTC().Format(time.RFC3339),
		"is_expired": current_time > expiryTime,
	}

	c.JSON(200, info)
}

func listHandle(c *gin.Context) {

	var allLinks []map[string]any

	allLinks, err := ListURLs()
	if err != nil {
		c.JSON(500, gin.H{"error": "Failed to list URLs"})
		return
	}
	c.JSON(200, allLinks)
}


func deleteHandle(c *gin.Context) {
	code := c.Param("code")

	err := DeleteURL(code)
    if err != nil {
        c.JSON(http.StatusNotFound, gin.H{"error": "Short URL not found or could not be deleted"})
        return
    }

	c.Status(http.StatusNoContent)
}

func rateLimitMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		ip := c.ClientIP()

		rlMutex.Lock()
		limiter, exists := rateLimiters[ip]
		if !exists || time.Since(limiter.lastRequest) > rateLimitWindow {
			limiter = &rateLimiter{
				lastRequest: time.Now(),
				requests: 1,
			}

			rateLimiters[ip] = limiter
			rlMutex.Unlock()
			return
		}

		if limiter.requests >= maxRequests {
			rlMutex.Unlock()
			c.AbortWithStatusJSON(429, gin.H{"error": "Rate limit exceeded. Try again later."})
			return
		}

		limiter.requests++
		limiter.lastRequest = time.Now()
		rlMutex.Unlock()
	}
}

func main() {
	router := gin.Default()

	router.POST("/shorten", rateLimitMiddleware(), shortenHandler)
	router.GET("/:code", handleRedirects)
	router.GET("/info/:code", infoHandler)
	router.GET("/list", listHandle)
	router.DELETE("/delete/:code", deleteHandle)

	srv := &http.Server{
		Addr: ":8080",
		Handler: router,
	}

	go func(){
		for {
			time.Sleep(time.Minute)
			cleanRateLimiters()
		}
	}()

	stopCleanup := make(chan struct{})
	go startCleanupTicker(stopCleanup)

	go func(){
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("listen: %s\n", err)
		}
	}()
    log.Println("Server is running at :8080")

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, os.Interrupt)
	<-quit

	fmt.Println("Shutdown Server ...")

	close(stopCleanup)

	ctx, cancel := context.WithTimeout(context.Background(), 5 * time.Second)
	defer cancel()
	
    if err := srv.Shutdown(ctx); err != nil {
        log.Fatal("Server Shutdown:", err)
    }

	log.Println("Server exiting")
}

func cleanRateLimiters() {
	rlMutex.Lock()
	defer rlMutex.Unlock()

	now := time.Now()

	for ip, rl := range rateLimiters {
		if now.Sub(rl.lastRequest) > 2 * time.Minute {
			delete(rateLimiters, ip)
		}
	}
}

func startCleanupTicker(stop <-chan struct{}) {
    ticker := time.NewTicker(24 * time.Hour)
    defer ticker.Stop()

    for {
        select {
        case <-ticker.C:
            cleanUpExpiredLinks()
        case <-stop:
            log.Println("Cleanup ticker stopped.")
            return
        }
    }
}
