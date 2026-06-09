package database_test

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/shareinto/paas/internal/platform/database"
)

func TestConfigDSNUsesMySQLUTF8MB4Defaults(t *testing.T) {
	cfg := database.DefaultConfig()
	cfg.Password = "secret"
	cfg.Timeout = 3 * time.Second

	dsn := cfg.DSN()
	for _, want := range []string{
		"paas:secret@tcp(127.0.0.1:3306)/paas?",
		"charset=utf8mb4",
		"collation=utf8mb4_unicode_ci",
		"parseTime=true",
		"timeout=3s",
	} {
		if !strings.Contains(dsn, want) {
			t.Fatalf("DSN %q does not contain %q", dsn, want)
		}
	}
}

func TestOpenReturnsPingError(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
	defer cancel()

	_, err := database.Open(ctx, database.Config{
		Host:      "127.0.0.1",
		Port:      1,
		Database:  "paas",
		User:      "paas",
		Password:  "bad",
		ParseTime: true,
		Location:  "Local",
		Timeout:   time.Millisecond,
	})
	if err == nil {
		t.Fatalf("Open() should return ping error for unavailable MySQL")
	}
}
