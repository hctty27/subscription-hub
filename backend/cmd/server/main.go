package main

import (
	"errors"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/hctty27/subscription-hub/backend/internal/httpapi"
	"github.com/hctty27/subscription-hub/backend/internal/store"
)

func main() {
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}
	host := os.Getenv("HOST")
	if host == "" {
		host = "127.0.0.1"
	}
	timezone := os.Getenv("APP_TIMEZONE")
	if timezone == "" {
		timezone = "Asia/Shanghai"
	}
	loc, err := time.LoadLocation(timezone)
	if err != nil {
		log.Fatalf("无效的 APP_TIMEZONE：%v", err)
	}
	repo, err := store.Open(os.Getenv("DB_PATH"))
	if err != nil {
		log.Fatalf("数据库初始化失败：%v", err)
	}
	defer repo.Close()

	server := &http.Server{
		Addr:              host + ":" + port,
		Handler:           httpapi.NewRouter(repo, loc),
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       15 * time.Second,
		WriteTimeout:      15 * time.Second,
		IdleTimeout:       60 * time.Second,
	}

	log.Printf("API 服务已启动，地址 %s，时区 %s", server.Addr, timezone)
	if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		log.Fatal(err)
	}
}
