package store

import (
	"database/sql"
	"embed"
	"fmt"
	"io/fs"
	"sort"
	"strings"
	"time"
)

//go:embed migrations/postgres/*.sql migrations/sqlite/*.sql
var migrationFS embed.FS

func (s *SQLStore) runMigrations() error {
	if _, err := s.db.Exec(`
CREATE TABLE IF NOT EXISTS schema_migrations (
  name TEXT PRIMARY KEY,
  applied_at TIMESTAMPTZ NOT NULL
)`); err != nil {
		return fmt.Errorf("create schema_migrations table: %w", err)
	}

	dialectDir := "migrations/sqlite"
	if s.isPostgres() {
		dialectDir = "migrations/postgres"
	}

	files, err := fs.ReadDir(migrationFS, dialectDir)
	if err != nil {
		return fmt.Errorf("read migrations dir %s: %w", dialectDir, err)
	}
	sort.Slice(files, func(i, j int) bool {
		return files[i].Name() < files[j].Name()
	})

	tx, err := s.db.Begin()
	if err != nil {
		return fmt.Errorf("begin migration tx: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	ph := s.placeholder
	lookupQuery := fmt.Sprintf("SELECT 1 FROM schema_migrations WHERE name = %s", ph(1))
	insertQuery := fmt.Sprintf("INSERT INTO schema_migrations (name, applied_at) VALUES (%s, %s)", ph(1), ph(2))

	for _, f := range files {
		if f.IsDir() || !strings.HasSuffix(f.Name(), ".sql") {
			continue
		}

		name := f.Name()
		var marker int
		err := tx.QueryRow(lookupQuery, name).Scan(&marker)
		if err == nil {
			continue
		}
		if err != nil && err != sql.ErrNoRows {
			return fmt.Errorf("check migration %s: %w", name, err)
		}

		path := fmt.Sprintf("%s/%s", dialectDir, name)
		sqlBytes, err := migrationFS.ReadFile(path)
		if err != nil {
			return fmt.Errorf("read migration file %s: %w", path, err)
		}

		if _, err := tx.Exec(string(sqlBytes)); err != nil {
			return fmt.Errorf("apply migration %s: %w", name, err)
		}
		if _, err := tx.Exec(insertQuery, name, time.Now().UTC()); err != nil {
			return fmt.Errorf("record migration %s: %w", name, err)
		}
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit migration tx: %w", err)
	}
	return nil
}
