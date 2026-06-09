package database

import (
	"context"
	"database/sql"
	"fmt"
	"net/url"
	"os"
	"strconv"
	"time"

	_ "github.com/go-sql-driver/mysql"
)

type Config struct {
	Host      string
	Port      int
	Database  string
	User      string
	Password  string
	ParseTime bool
	Location  string
	Timeout   time.Duration
}

func DefaultConfig() Config {
	return Config{
		Host:      "127.0.0.1",
		Port:      3306,
		Database:  "paas",
		User:      "paas",
		ParseTime: true,
		Location:  "Local",
		Timeout:   5 * time.Second,
	}
}

func ConfigFromEnv() Config {
	cfg := DefaultConfig()
	if v := os.Getenv("MYSQL_HOST"); v != "" {
		cfg.Host = v
	}
	if v := os.Getenv("MYSQL_PORT"); v != "" {
		if port, err := strconv.Atoi(v); err == nil {
			cfg.Port = port
		}
	}
	if v := os.Getenv("MYSQL_DATABASE"); v != "" {
		cfg.Database = v
	}
	if v := os.Getenv("MYSQL_USER"); v != "" {
		cfg.User = v
	}
	if v := os.Getenv("MYSQL_PASSWORD"); v != "" {
		cfg.Password = v
	}
	return cfg
}

func (c Config) DSN() string {
	values := url.Values{}
	values.Set("charset", "utf8mb4")
	values.Set("collation", "utf8mb4_unicode_ci")
	values.Set("parseTime", strconv.FormatBool(c.ParseTime))
	values.Set("loc", c.Location)
	values.Set("timeout", c.Timeout.String())
	values.Set("interpolateParams", "true")
	return fmt.Sprintf("%s:%s@tcp(%s:%d)/%s?%s", c.User, c.Password, c.Host, c.Port, c.Database, values.Encode())
}

func Open(ctx context.Context, cfg Config) (*sql.DB, error) {
	db, err := sql.Open("mysql", cfg.DSN())
	if err != nil {
		return nil, err
	}
	if err := db.PingContext(ctx); err != nil {
		_ = db.Close()
		return nil, err
	}
	return db, nil
}
