package database

import (
	"context"
	"embed"
	"fmt"
	"log/slog"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/jmoiron/sqlx"
	"github.com/topi314/gomigrate"
	"github.com/topi314/gomigrate/drivers/sqlite"
)

//go:embed migrations/*.sql
var migrations embed.FS

func New(cfg Config) (*Database, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	dbx, err := sqlx.ConnectContext(ctx, "pgx", cfg.DataSourceName())
	if err != nil {
		return nil, fmt.Errorf("failed to connect to database: %w", err)
	}

	if err = gomigrate.Migrate(ctx, dbx, sqlite.New, migrations); err != nil {
		return nil, fmt.Errorf("failed to run migrations: %w", err)
	}

	db := &Database{
		db: dbx,
	}

	go db.cleanup()

	return db, nil
}

type Database struct {
	db *sqlx.DB
}

func (d *Database) Close() error {
	if err := d.db.Close(); err != nil {
		return fmt.Errorf("failed to close database connection: %w", err)
	}
	return nil
}

func (d *Database) cleanup() {
	for {
		d.doCleanupSessions()
		d.doCleanupCampfireTokens()
		time.Sleep(5 * time.Minute)
	}
}

func (d *Database) doCleanupSessions() {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := d.DeleteExpiredSessions(ctx); err != nil {
		slog.Error("failed to cleanup expired sessions", slog.Any("err", err))
	}
}

func (d *Database) doCleanupCampfireTokens() {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := d.DeleteExpiredCampfireTokens(ctx); err != nil {
		slog.Error("failed to cleanup expired campfire tokens", slog.Any("err", err))
	}
}
