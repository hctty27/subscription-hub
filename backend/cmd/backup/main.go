package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"time"

	"github.com/hctty27/subscription-hub/backend/internal/store"
)

func main() {
	dir := os.Getenv("BACKUP_DIR")
	if dir == "" {
		dir = filepath.Join("data", "backups")
	}

	repo, err := store.Open(os.Getenv("DB_PATH"))
	if err != nil {
		log.Fatalf("数据库连接失败：%v", err)
	}
	defer repo.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	result, err := repo.Backup(ctx, dir, 7)
	if err != nil {
		log.Fatalf("创建数据库快照失败：%v", err)
	}
	fmt.Printf("数据库备份已完成：%s（%d 字节）\n", result.File, result.Size)
}
