package postgres

import (
	"database/sql"
	"fmt"
	"io/fs"
	"log/slog"

	"github.com/pressly/goose/v3"
	_ "github.com/jackc/pgx/v5/stdlib" // pgx driver for database/sql
)

// RunMigrations runs all pending database migrations using the provided filesystem.
// The migrationsFS should contain SQL files at its root (e.g., via embed.FS).
// The dir parameter is the subdirectory within the FS where migrations live.
func RunMigrations(databaseURL string, migrationsFS fs.FS, dir string) error {
	db, err := sql.Open("pgx", databaseURL)
	if err != nil {
		return fmt.Errorf("open database for migrations: %w", err)
	}
	defer db.Close()

	goose.SetBaseFS(migrationsFS)

	if err := goose.SetDialect("postgres"); err != nil {
		return fmt.Errorf("set dialect: %w", err)
	}

	if err := goose.Up(db, dir); err != nil {
		return fmt.Errorf("run migrations: %w", err)
	}

	slog.Info("database migrations complete")
	return nil
}
