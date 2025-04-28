package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"
	"sync"
	"regexp"
	"time"
)

var (
	idCounter int64
	mutex     sync.Mutex
	base62    = "0123456789abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ"
	filename  = "store.json"
	validCodeRegex = regexp.MustCompile(`^[a-zA-Z0-9]+$`)
)

type URLData struct {
	LongURL string `json:"long_url"`
	Clicks int `json:"clicks"`
	CreatedAt int64  `json:"created_at"`
    Expiry    int64  `json:"expiry"` 
}

var urlStore = make(map[string]URLData)

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

func saveStore() {
	data := Store{
		IDCounter: idCounter,
		URLStore: urlStore,
	}

	fileBytes, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		log.Fatalf("Error marshaling JSON: %v", err)
	}

	tempFile := filename + ".tmp"
	err = os.WriteFile(tempFile, fileBytes, 0644)
	if err != nil {
		log.Fatalf("Error writing temp file: %v", err)
	}

	err = os.Rename(tempFile, filename)
	if err != nil {
		log.Fatalf("Error renaming temp file: %v", err)
	}
}

func loadStore(){
	if _, err := os.Stat(filename); os.IsNotExist(err) {
		fmt.Println("No existing store file. Starting fresh.")
		return
	}

	data, err := os.ReadFile(filename)
	if err != nil {
		log.Fatalf("Error reading store file: %v", err)
	}

	var store Store
	if err := json.Unmarshal(data, &store); err != nil {
		log.Fatalf("Error unmarshaling JSON: %v", err)
	}

	idCounter = store.IDCounter
	urlStore = store.URLStore
	fmt.Println("Loaded store with", len(urlStore), "entries.")
}

func cleanUpExpiredLinks() {
	mutex.Lock()
	defer mutex.Unlock()

	now := time.Now().Unix()

	for code, data := range urlStore {
		if now > data.CreatedAt + data.Expiry {
			delete(urlStore, code)
		}
	}

	saveStore()
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

func shortenHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Only POST allowed", http.StatusMethodNotAllowed)
		return
	}

	var body struct {
		URL        string `json:"url"`
		CustomCode string `json:"custom_code,omitempty"`
		ExpirySeconds  int64  `json:"expiry_seconds,omitempty"`
	}

	err := json.NewDecoder(r.Body).Decode(&body)
	if err != nil || body.URL == "" {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	if !isValidURL(body.URL) {
		http.Error(w, "Invalid URL. Must start with http:// or https://", http.StatusBadRequest)
		return
	}

	mutex.Lock()
	defer mutex.Unlock()

	var code string
	if body.CustomCode != "" {
		if !isValidCode(body.CustomCode) {
			http.Error(w, "Invalid custom code. Use only letters and numbers.", http.StatusBadRequest)
			return
		}
		if _, exists := urlStore[body.CustomCode]; exists {
			http.Error(w, "Custom code already in use", http.StatusConflict)
			return
		}
		code = body.CustomCode
	} else {
		idCounter++
		code = encodeBase62(idCounter)
	}

	expiry := body.ExpirySeconds
	if expiry == 0 {
		expiry = 7 * 24 * 3600 // Default 7 days
	}

	urlStore[code] = URLData{
		LongURL: body.URL,
		Clicks: 0,
		CreatedAt: time.Now().Unix(),
		Expiry: expiry, // 7 days in seconds
	}

	saveStore()

	shortURL := fmt.Sprintf("http://localhost:8080/%s", code)
	json.NewEncoder(w).Encode(map[string]any{
		"short_url": shortURL,
		"expiry_seconds": expiry,
	})
}

func handleRedirects(w http.ResponseWriter, r *http.Request) {
	code := strings.TrimPrefix(r.URL.Path, "/")

	now := time.Now().Unix()

	if data, ok := urlStore[code]; ok {
		if data.Expiry != 0 && now > data.CreatedAt+data.Expiry {
			http.Error(w, "URL expired", http.StatusGone)
			return
		}
	}

	if data, ok := urlStore[code]; ok{
		data.Clicks++
		urlStore[code] = data
		saveStore()
		http.Redirect(w, r, data.LongURL, http.StatusFound)
	}else {
		http.Error(w, "URL not found!", http.StatusNotFound)
	}
}

func infoHandler(w http.ResponseWriter, r *http.Request) {
	code := strings.TrimPrefix(r.URL.Path, "/info/")

	mutex.Lock()
	defer mutex.Unlock()

	data, exists := urlStore[code]
	if !exists {
		http.Error(w, "Short URL not found", http.StatusNotFound)
		return
	}

	current_time := time.Now().Unix()
	expiryTime := data.CreatedAt + data.Expiry

	info := map[string]any{
		"long_url": data.LongURL,
		"clicks": data.Clicks,
		"created_at": time.Unix(data.CreatedAt, 0).UTC().Format(time.RFC3339),
		"expires_at": time.Unix(data.CreatedAt+data.Expiry, 0).UTC().Format(time.RFC3339),
		"is_expired": current_time > expiryTime,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(info)
}

func listHandle(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Only GET allowed", http.StatusMethodNotAllowed)
		return
	}

	mutex.Lock()
	defer mutex.Unlock()

	var allLinks []map[string]any

	current_time := time.Now().Unix()

	for code, data := range urlStore {
		expiryTime := data.CreatedAt + data.Expiry
		allLinks = append(allLinks, map[string]any{
			"code": code,
			"long_url": data.LongURL,
			"clicks": data.Clicks,
			"created_at": time.Unix(data.CreatedAt, 0).UTC().Format(time.RFC3339),
			"expires_at": time.Unix(data.CreatedAt+data.Expiry, 0).UTC().Format(time.RFC3339),
			"is_expired": current_time > expiryTime, 
		})
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(allLinks)
}

func deleteHandle(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodDelete {
		http.Error(w, "Only DELETE allowed", http.StatusMethodNotAllowed)
		return
	}

	code := strings.TrimPrefix(r.URL.Path, "/delete/")

	mutex.Lock()
	defer mutex.Unlock()

	if _, exists := urlStore[code]; !exists{
		http.Error(w, "Short URL not found", http.StatusNotFound)
		return
	}

	delete(urlStore, code)
	saveStore()

	w.WriteHeader(http.StatusNoContent)
}

func main() {
	loadStore()
	
	go func() {
		ticker := time.NewTicker(24 * time.Hour)
		defer ticker.Stop()

		for {
			<-ticker.C
			cleanUpExpiredLinks()
		}
	}()

	http.HandleFunc("/shorten", shortenHandler)
	http.HandleFunc("/info/", infoHandler)
	http.HandleFunc("/list", listHandle)
	http.HandleFunc("/delete/", deleteHandle)
	http.HandleFunc("/", handleRedirects)

	fmt.Println("Server is running at :8080")
	log.Fatal(http.ListenAndServe(":8080", nil))
}
