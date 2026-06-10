package testsupport

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/docker/go-connections/nat"
	mysqlDriver "github.com/go-sql-driver/mysql"
	"github.com/shareinto/paas/internal/platform/database"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
)

type MySQLDatabase struct {
	DB       *sql.DB
	Host     string
	Port     string
	Database string
	User     string
	Password string
}

var mysqlContainer struct {
	sync.Mutex
	db  *sql.DB
	dsn string
}

func MySQLDB(t *testing.T, migrations ...database.Migration) *sql.DB {
	t.Helper()
	return MySQLDatabaseForTest(t, migrations...).DB
}

func MySQLDatabaseForTest(t *testing.T, migrations ...database.Migration) MySQLDatabase {
	t.Helper()

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	t.Cleanup(cancel)

	rootDB := sharedMySQLDB(t, ctx)
	dbName := strings.ToLower(strings.ReplaceAll(t.Name(), "/", "_"))
	dbName = "paas_test_" + sanitizeDatabaseName(dbName)
	if len(dbName) > 60 {
		dbName = dbName[:60]
	}
	if _, err := rootDB.ExecContext(ctx, "DROP DATABASE IF EXISTS "+dbName); err != nil {
		t.Fatalf("drop test database: %v", err)
	}
	if _, err := rootDB.ExecContext(ctx, "CREATE DATABASE "+dbName+" CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci"); err != nil {
		t.Fatalf("create test database: %v", err)
	}
	t.Cleanup(func() { _, _ = rootDB.ExecContext(context.Background(), "DROP DATABASE IF EXISTS "+dbName) })

	cfg, err := mysqlDriver.ParseDSN(mysqlContainer.dsn)
	if err != nil {
		t.Fatalf("parse mysql test dsn: %v", err)
	}
	cfg.DBName = dbName
	dsn := cfg.FormatDSN()
	db, err := sql.Open("mysql", dsn)
	if err != nil {
		t.Fatalf("sql.Open() error = %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	if err := db.PingContext(ctx); err != nil {
		t.Fatalf("PingContext() error = %v", err)
	}
	if len(migrations) > 0 {
		if err := database.NewMigrator(db).Up(ctx, migrations); err != nil {
			t.Fatalf("migrate test database: %v", err)
		}
	}
	host, port := splitTCPAddr(cfg.Addr)
	return MySQLDatabase{DB: db, Host: host, Port: port, Database: dbName, User: cfg.User, Password: cfg.Passwd}
}

func ConfigureMySQLEnv(t *testing.T, migrations ...database.Migration) *sql.DB {
	t.Helper()
	testDB := MySQLDatabaseForTest(t, migrations...)
	t.Setenv("MYSQL_HOST", testDB.Host)
	t.Setenv("MYSQL_PORT", testDB.Port)
	t.Setenv("MYSQL_DATABASE", testDB.Database)
	t.Setenv("MYSQL_USER", testDB.User)
	t.Setenv("MYSQL_PASSWORD", testDB.Password)
	return testDB.DB
}

func sharedMySQLDB(t *testing.T, ctx context.Context) *sql.DB {
	t.Helper()
	mysqlContainer.Lock()
	defer mysqlContainer.Unlock()
	if mysqlContainer.db != nil {
		return mysqlContainer.db
	}
	if dsn := os.Getenv("PAAS_TEST_MYSQL_DSN"); dsn != "" {
		db, err := sql.Open("mysql", dsn)
		if err != nil {
			t.Fatalf("sql.Open() error = %v", err)
		}
		if err := db.PingContext(ctx); err != nil {
			t.Fatalf("PingContext() error = %v", err)
		}
		mysqlContainer.db = db
		mysqlContainer.dsn = dsn
		return db
	}
	image := os.Getenv("PAAS_TEST_MYSQL_IMAGE")
	if image == "" {
		image = "m.daocloud.io/docker.io/library/mysql:8.0"
	}
	container, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: testcontainers.ContainerRequest{
			Image:        image,
			ExposedPorts: []string{"3306/tcp"},
			Env: map[string]string{
				"MYSQL_ROOT_PASSWORD": "password",
			},
			WaitingFor: wait.ForLog("port: 3306  MySQL Community Server").WithStartupTimeout(90 * time.Second),
		},
		Started: true,
	})
	if err != nil {
		t.Skipf("skip MySQL-backed test: start mysql container: %v", err)
	}

	host, err := container.Host(ctx)
	if err != nil {
		t.Fatalf("mysql host: %v", err)
	}
	port, err := container.MappedPort(ctx, nat.Port("3306/tcp"))
	if err != nil {
		t.Fatalf("mysql mapped port: %v", err)
	}
	dsn := fmt.Sprintf("root:password@tcp(%s:%s)/mysql?charset=utf8mb4&collation=utf8mb4_unicode_ci&parseTime=true&loc=Local&interpolateParams=true", host, port.Port())
	db, err := sql.Open("mysql", dsn)
	if err != nil {
		t.Fatalf("sql.Open() error = %v", err)
	}
	if err := db.PingContext(ctx); err != nil {
		if strings.Contains(strings.ToLower(err.Error()), "docker") {
			t.Skipf("skip MySQL-backed test: %v", err)
		}
		t.Fatalf("PingContext() error = %v", err)
	}
	mysqlContainer.db = db
	mysqlContainer.dsn = dsn
	return db
}

func sanitizeDatabaseName(name string) string {
	var b strings.Builder
	for _, r := range name {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '_' {
			b.WriteRune(r)
			continue
		}
		b.WriteByte('_')
	}
	return b.String()
}

func splitTCPAddr(addr string) (string, string) {
	host, port, ok := strings.Cut(addr, ":")
	if !ok {
		return addr, "3306"
	}
	return host, port
}
