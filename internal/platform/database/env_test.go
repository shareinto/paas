package database_test

import (
	"strings"
	"testing"

	"github.com/shareinto/paas/internal/platform/database"
)

func TestConfigFromEnv(t *testing.T) {
	t.Setenv("MYSQL_HOST", "mysql.internal")
	t.Setenv("MYSQL_PORT", "3307")
	t.Setenv("MYSQL_DATABASE", "paas_test")
	t.Setenv("MYSQL_USER", "tester")
	t.Setenv("MYSQL_PASSWORD", "secret")

	cfg := database.ConfigFromEnv()
	if cfg.Host != "mysql.internal" || cfg.Port != 3307 || cfg.Database != "paas_test" || cfg.User != "tester" || cfg.Password != "secret" {
		t.Fatalf("unexpected config from env: %+v", cfg)
	}
	if !strings.Contains(cfg.DSN(), "tester:secret@tcp(mysql.internal:3307)/paas_test") {
		t.Fatalf("unexpected dsn: %s", cfg.DSN())
	}
}
