package main

import (
	"context"
	"errors"
	"path/filepath"
	"log"
	"net/http"
	"os"
	"time"
	_ "time/tzdata"

	"github.com/hctty27/subscription-hub/backend/internal/httpapi"
	"github.com/hctty27/subscription-hub/backend/internal/auth"
	"github.com/hctty27/subscription-hub/backend/internal/notify"
	"github.com/hctty27/subscription-hub/backend/internal/reminder"
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
	security,err:=auth.New(context.Background(),repo,os.Getenv("ADMIN_PASSWORD"),auth.SecureCookieFromEnvironment())
	if err!=nil{log.Fatalf("管理员身份认证初始化失败：%v",err)}
	backupDir:=os.Getenv("BACKUP_DIR")
	if backupDir==""{backupDir=filepath.Join("data","backups")}
	workerContext,cancelWorker:=context.WithCancel(context.Background())
	defer cancelWorker()
	go startBackupWorker(workerContext,repo,backupDir,loc)
	sender,err:=notify.FromEnv()
	if err!=nil{log.Fatalf("通知渠道配置无效：%v",err)}
	dispatcher:=&notify.Dispatcher{Repo:repo,Sender:sender,Location:loc}
	log.Printf("外部通知渠道：%s",notify.DeliverySummary(sender.Channels()))
	go reminder.Start(workerContext, repo, loc, dispatcher)

	server := &http.Server{
		Addr:              host + ":" + port,
		Handler:           httpapi.NewRouter(repo, loc, security, backupDir, dispatcher),
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

func startBackupWorker(ctx context.Context,repo *store.Store,dir string,loc *time.Location){
  for {
    now:=time.Now().In(loc)
    next:=time.Date(now.Year(),now.Month(),now.Day(),3,0,0,0,loc)
    if !next.After(now){next=next.AddDate(0,0,1)}
    timer:=time.NewTimer(time.Until(next))
    select{
    case <-ctx.Done():timer.Stop();return
    case <-timer.C:
      backupCtx,cancel:=context.WithTimeout(ctx,2*time.Minute)
      info,err:=repo.Backup(backupCtx,dir,7)
      cancel()
      if err!=nil{log.Printf("自动备份失败：%v",err)}else{log.Printf("自动备份完成：%s",info.File)}
    }
  }
}
