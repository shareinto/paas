package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/shareinto/paas/internal/migrations"
	"github.com/shareinto/paas/internal/platform/database"
)

func main() {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := migrateDatabaseIfEnabled(ctx); err != nil {
		log.Fatalf("执行 MySQL 迁移失败: %v", err)
	}

	app, err := newApplication(ctx)
	if err != nil {
		log.Fatalf("初始化 PaaS 控制面失败: %v", err)
	}

	addr := os.Getenv("PAAS_HTTP_ADDR")
	if addr == "" {
		addr = ":8080"
	}
	log.Printf("PaaS 控制面已启动: http://127.0.0.1%s", addr)
	if err := http.ListenAndServe(addr, app.handler); err != nil {
		log.Fatalf("PaaS 控制面退出: %v", err)
	}
}

func migrateDatabaseIfEnabled(ctx context.Context) error {
	if os.Getenv("PAAS_AUTO_MIGRATE") != "true" {
		return nil
	}
	db, err := database.Open(ctx, database.ConfigFromEnv())
	if err != nil {
		return err
	}
	defer db.Close()
	return database.NewMigrator(db).Up(ctx, migrations.All())
}
