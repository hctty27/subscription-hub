package main

import (
	"log"
	"net/http"
	"os"
	"time"

	"github.com/hctty27/subscription-hub/backend/internal/httpapi"
)

func main() {
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	server := &http.Server{
		Addr:              ":" + port,
		Handler:           httpapi.NewRouter(),
		ReadHeaderTimeout: 5 * time.Second,
	}

	log.Printf("API 服务已启动，监听端口 %s", port)
	if err := server.ListenAndServe(); err != nil {
		log.Fatal(err)
	}
}
