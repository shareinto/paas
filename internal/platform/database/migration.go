package database

import (
	"context"
	"database/sql"
	"fmt"
	"sort"
	"strings"
	"time"
)

type Migration struct {
	Version int64
	Name    string
	Up      string
	Down    string
}

type Migrator struct {
	db *sql.DB
}

func NewMigrator(db *sql.DB) *Migrator {
	return &Migrator{db: db}
}

func (m *Migrator) Up(ctx context.Context, migrations []Migration) error {
	if err := m.ensureTable(ctx); err != nil {
		return err
	}

	ordered := append([]Migration(nil), migrations...)
	sort.Slice(ordered, func(i, j int) bool { return ordered[i].Version < ordered[j].Version })

	for _, migration := range ordered {
		applied, err := m.isApplied(ctx, migration.Version)
		if err != nil {
			return err
		}
		if applied {
			continue
		}
		if err := m.applyUp(ctx, migration); err != nil {
			return err
		}
	}
	return nil
}

func (m *Migrator) Down(ctx context.Context, migrations []Migration) error {
	if err := m.ensureTable(ctx); err != nil {
		return err
	}

	ordered := append([]Migration(nil), migrations...)
	sort.Slice(ordered, func(i, j int) bool { return ordered[i].Version > ordered[j].Version })

	for _, migration := range ordered {
		applied, err := m.isApplied(ctx, migration.Version)
		if err != nil {
			return err
		}
		if !applied {
			continue
		}
		if err := m.applyDown(ctx, migration); err != nil {
			return err
		}
	}
	return nil
}

func (m *Migrator) ensureTable(ctx context.Context) error {
	_, err := m.db.ExecContext(ctx, `
CREATE TABLE IF NOT EXISTS schema_migrations (
  version BIGINT NOT NULL PRIMARY KEY,
  name VARCHAR(255) NOT NULL,
  applied_at DATETIME(6) NOT NULL
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci`)
	return err
}

func (m *Migrator) isApplied(ctx context.Context, version int64) (bool, error) {
	var found int
	err := m.db.QueryRowContext(ctx, "SELECT 1 FROM schema_migrations WHERE version = ?", version).Scan(&found)
	if err == nil {
		return true, nil
	}
	if err == sql.ErrNoRows {
		return false, nil
	}
	return false, err
}

func (m *Migrator) applyUp(ctx context.Context, migration Migration) error {
	if migration.Up == "" {
		return fmt.Errorf("migration %d has empty up sql", migration.Version)
	}
	tx, err := m.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()

	if err := execScript(ctx, tx, migration.Up); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx,
		"INSERT INTO schema_migrations(version, name, applied_at) VALUES (?, ?, ?)",
		migration.Version,
		migration.Name,
		time.Now().UTC(),
	); err != nil {
		return err
	}
	return tx.Commit()
}

func (m *Migrator) applyDown(ctx context.Context, migration Migration) error {
	if migration.Down == "" {
		return fmt.Errorf("migration %d has empty down sql", migration.Version)
	}
	tx, err := m.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()

	if err := execScript(ctx, tx, migration.Down); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, "DELETE FROM schema_migrations WHERE version = ?", migration.Version); err != nil {
		return err
	}
	return tx.Commit()
}

func execScript(ctx context.Context, tx *sql.Tx, script string) error {
	statements := splitSQLStatements(script)
	if len(statements) == 0 {
		return fmt.Errorf("migration sql has no executable statements")
	}
	for _, statement := range statements {
		if _, err := tx.ExecContext(ctx, statement); err != nil {
			return err
		}
	}
	return nil
}

func splitSQLStatements(script string) []string {
	var statements []string
	var current strings.Builder
	var quote rune
	escaped := false

	for _, r := range script {
		if quote != 0 {
			current.WriteRune(r)
			if escaped {
				escaped = false
				continue
			}
			if r == '\\' {
				escaped = true
				continue
			}
			if r == quote {
				quote = 0
			}
			continue
		}

		switch r {
		case '\'', '"', '`':
			quote = r
			current.WriteRune(r)
		case ';':
			appendStatement(&statements, current.String())
			current.Reset()
		default:
			current.WriteRune(r)
		}
	}
	appendStatement(&statements, current.String())
	return statements
}

func appendStatement(statements *[]string, statement string) {
	statement = strings.TrimSpace(statement)
	if statement == "" {
		return
	}
	*statements = append(*statements, statement)
}
