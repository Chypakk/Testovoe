package database

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"net/url"
	"path/filepath"
	"time"

	"testovoe/internal/configs"

	_ "github.com/go-sql-driver/mysql"
	_ "github.com/jackc/pgx/v5/stdlib"

	"github.com/golang-migrate/migrate/v4"
	mysqlmigrate "github.com/golang-migrate/migrate/v4/database/mysql"
	pgmigrate "github.com/golang-migrate/migrate/v4/database/postgres"
	_ "github.com/golang-migrate/migrate/v4/source/file"
)

type Database struct {
	DB     *sql.DB
	Driver string
}

func NewDatabase(cfg configs.DatabaseConfig) (*Database, error) {
	driver := cfg.NormalizedDriver()

	var sqlDriverName string
	var dsn string

	switch driver {
	case "mysql":
		sqlDriverName = "mysql"
		dsn = fmt.Sprintf(
			"%s:%s@tcp(%s:%s)/%s?parseTime=true&multiStatements=true",
			cfg.User,
			cfg.Password,
			cfg.Host,
			cfg.Port,
			cfg.Name,
		)

	case "postgres":
		sqlDriverName = "pgx"

		u := url.URL{
			Scheme: "postgres",
			User:   url.UserPassword(cfg.User, cfg.Password),
			Host:   fmt.Sprintf("%s:%s", cfg.Host, cfg.Port),
			Path:   cfg.Name,
		}

		q := u.Query()
		q.Set("sslmode", cfg.SSLMode)
		u.RawQuery = q.Encode()

		dsn = u.String()

	default:
		return nil, fmt.Errorf("unsupported database driver: %s", cfg.Driver)
	}

	sqlDB, err := sql.Open(sqlDriverName, dsn)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := sqlDB.PingContext(ctx); err != nil {
		_ = sqlDB.Close()
		return nil, fmt.Errorf("failed to ping database: %w", err)
	}

	sqlDB.SetMaxOpenConns(25)
	sqlDB.SetMaxIdleConns(25)
	sqlDB.SetConnMaxLifetime(5 * time.Minute)

	return &Database{
		DB:     sqlDB,
		Driver: driver,
	}, nil
}

func (d *Database) Close() error {
	return d.DB.Close()
}

func (d *Database) GetDB() *sql.DB {
	return d.DB
}

func (d *Database) RunMigrations() error {
	return d.runMigrations(true)
}

func (d *Database) RollbackMigrations() error {
	return d.runMigrations(false)
}

func (d *Database) runMigrations(up bool) error {
	migrationsPath := filepath.Join(".", "migrations", d.Driver)
	sourceURL := "file://" + filepath.ToSlash(migrationsPath)

	switch d.Driver {
	case "postgres":
		driver, err := pgmigrate.WithInstance(d.DB, &pgmigrate.Config{})
		if err != nil {
			return fmt.Errorf("failed to create postgres migration driver: %w", err)
		}

		m, err := migrate.NewWithDatabaseInstance(
			sourceURL,
			d.Driver,
			driver,
		)
		if err != nil {
			return fmt.Errorf("failed to create migrate instance: %w", err)
		}
		// defer m.Close()

		return applyMigrations(m, up)

	case "mysql":
		driver, err := mysqlmigrate.WithInstance(d.DB, &mysqlmigrate.Config{})
		if err != nil {
			return fmt.Errorf("failed to create mysql migration driver: %w", err)
		}

		m, err := migrate.NewWithDatabaseInstance(
			sourceURL,
			d.Driver,
			driver,
		)
		if err != nil {
			return fmt.Errorf("failed to create migrate instance: %w", err)
		}
		// defer m.Close()

		return applyMigrations(m, up)

	default:
		return fmt.Errorf("unsupported migration driver: %s", d.Driver)
	}
}

func applyMigrations(m *migrate.Migrate, up bool) error {
	if up {
		if err := m.Up(); err != nil && !errors.Is(err, migrate.ErrNoChange) {
			return fmt.Errorf("failed to apply migrations: %w", err)
		}
		return nil
	}

	if err := m.Down(); err != nil && !errors.Is(err, migrate.ErrNoChange) {
		return fmt.Errorf("failed to rollback migrations: %w", err)
	}

	return nil
}
