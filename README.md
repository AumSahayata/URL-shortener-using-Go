# 📦 URL Shortener Service (Go + Gin + Redis)

A simple and efficient **URL shortening service** built with **Go**, **Gin web framework**, and **Redis** as a storage backend.

It allows users to shorten long URLs, manage custom short codes, track click counts, and set expiry times for shortened links.

---

## 🚀 Features

- **Shorten URLs** (auto-generated or with custom codes)
- **Set expiry times** (default 7 days, or custom expiry)
- **Track click counts** for each shortened link
- **Get info** about a shortened URL (created time, clicks, expiry)
- **List all URLs** stored
- **Delete** a shortened URL manually
- **Automatic cleanup** of expired links
- **Graceful shutdown** handling
- **Simple rate limiting** per IP address to prevent abuse
- **Redis** backend for fast storage
- **Local memory** (rate limiters) cleaned periodically

---

## 🛠 Tech Stack

- **Go** (Golang)
- **Gin** Web Framework
- **Redis** (Storage backend)

---

## 🏗 Architecture

- **Gin** handles HTTP routing and middleware
- **Redis** stores URL metadata (`LongURL`, `Clicks`, `CreatedAt`, `Expiry`)
- **Memory (map)** tracks client IP rate limits
- **Goroutines** and **tickers** handle background tasks:
  - Cleaning expired Redis entries
  - Cleaning old rate limiter data
- **Graceful Shutdown** ensures all processes stop safely when the server exits

---

Example request to shorten a URL:

```json
POST /shorten
{
  "url": "https://example.com",
  "custom_code": "mycode", // optional
  "expiry_seconds": 3600   // optional
}
```

## 🚀 Setup & Running the Project

This project can run in two modes:  
- Using **local in-memory + JSON persistence**.
- Using **Redis** for persistence (external Redis server required).

---

### 🛠️ Prerequisites

- [Go](https://golang.org/dl/) 1.18+ installed.
- Redis installed and running (only if using Redis mode).
- (Optional) [direnv](https://direnv.net/) or manually export environment variables.

---

### 🗂️ Running in Local JSON Mode (default)

1. **Clone the repository**:

    ```bash
    git clone https://github.com/AumSahayata/URL-shortener-using-Go
    cd URL-shortener-using-Go/using-json
    ```

2. **Run the project**:

    ```bash
    go run main.go
    ```

3. **Access the app**:

    Open your browser at [http://localhost:8080](http://localhost:8080)

---

### 🛢️ Running in Redis Mode

1. **Clone the repository**:

    ```bash
    git clone https://github.com/AumSahayata/URL-shortener-using-Go
    cd URL-shortener-using-Go/using-redis
    ```

2. **Set up a Redis server** (locally or cloud hosted like Redis Cloud).

3. **Create a `.env` file** inside your project directory with the following content:

    ```env
    REDIS_ADDR=localhost:6379 (OR cloud database address)
    REDIS_USER=redis_user      # leave empty if no username
    REDIS_PASSWORD=redis_password   # set password if needed
    REDIS_DB=0        # Redis database number
    ```

4. **Install dependencies** (if not already):

    ```bash
    go mod tidy
    ```

5. **Run the project**:

    ```bash
    go run .
    ```

6. **Access the app**:

    Open your browser at [http://localhost:8080](http://localhost:8080)

---

### 📌 API Endpoints Overview

| Method | Endpoint             | Description                         |
| :----: | :-------------------- | :---------------------------------: |
| POST   | `/shorten`             | Shorten a new URL                  |
| GET    | `/:code`               | Redirect to original URL           |
| GET    | `/info/:code`          | Get details about a short URL      |
| GET    | `/list`                | List all URLs                      |
| DELETE | `/delete/:code`        | Delete a shortened URL             |

---

### 📋 Notes

- Rate limiting: Max **5 requests/minute** per IP.
- Expired links are automatically cleaned every 24 hours.
- Graceful shutdown is implemented (CTRL+C to terminate cleanly).
- Default short URL expiry is **7 days**, customizable per link.

---


## 🙌 Contributing

Pull requests are welcome!  
Feel free to fork the project, open issues, and submit PRs!  
Any suggestions to improve it further are appreciated.

---

## 📢 Author

Built with ❤️ by Aum Sahayata.
