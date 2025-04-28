package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"strconv"
	"time"

	"github.com/joho/godotenv"
	"github.com/redis/go-redis/v9"
)

var (
	
	Ctx = context.Background()
    Rdb *redis.Client
)


func init() {
    err := godotenv.Load()
    if err != nil {
        log.Println("No .env file found, using system environment variables")
    }

	db, _ := strconv.Atoi(os.Getenv("REDIS_DB"))
	fmt.Println(os.Getenv("REDIS_ADDR"))

    Rdb = redis.NewClient(&redis.Options{
        Addr:     os.Getenv("REDIS_ADDR"),
		Username: os.Getenv("REDIS_USER"),
        Password: os.Getenv("REDIS_PASSWORD"),
        DB: db,
    })

    _, err = Rdb.Ping(Ctx).Result()
    if err != nil {
        log.Fatalf("Failed to connect to Redis: %v", err)
    }
    fmt.Println("Connected to Redis successfully.")
}

func SaveURL(code string, data URLData) error {
	jsonData, err := json.Marshal(data)
	if err != nil {
		return err
	}

	err = Rdb.Set(Ctx, code, jsonData, 0).Err() // 0 expiry means "no expiry", we handle expiry ourselves
	return err
}

func GetURL(code string) (URLData, error) {
	val, err := Rdb.Get(Ctx, code).Result()
	if err != nil {
		return URLData{}, err
	}

	var data URLData
	err = json.Unmarshal([]byte(val), &data)
	return data, err
}

func DeleteURL(code string) error {
	return Rdb.Del(Ctx, code).Err()
}

func ListURLs() ([]map[string]any, error) {
	var results []map[string]any

	iter := Rdb.Scan(Ctx, 0, "*", 0).Iterator()
	for iter.Next(Ctx) {
		key := iter.Val()
		data, err := GetURL(key)
		if err != nil {
			continue
		}

		current_time := time.Now().Unix()
		expiryTime := data.CreatedAt + data.Expiry

		results = append(results, map[string]any{
			"code":       key,
			"long_url":   data.LongURL,
			"clicks":     data.Clicks,
			"created_at": time.Unix(data.CreatedAt, 0).UTC().Format(time.RFC3339),
			"expires_at": time.Unix(expiryTime, 0).UTC().Format(time.RFC3339),
			"is_expired": current_time > expiryTime,
		})
	}
	if err := iter.Err(); err != nil {
		return nil, err
	}

	return results, nil
}

func GetNextID() (int64, error) {
	return Rdb.Incr(Ctx, "url_id_counter").Result()
}
