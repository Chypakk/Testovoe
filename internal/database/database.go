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
	"testovoe/internal/logger"

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

	logger.Info("Connecting to database", 
		"driver", driver,
		"host", cfg.Host,
		"port", cfg.Port,
		"db", cfg.Name,
	)

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
		err := fmt.Errorf("unsupported database driver: %s", cfg.Driver)
		logger.Error("Unsupported database driver", err,
			"driver", cfg.Driver,
		)
		return nil, err
	}

	sqlDB, err := sql.Open(sqlDriverName, dsn)
	if err != nil {
		err = fmt.Errorf("failed to open database: %w", err)
		logger.Error("Failed to open database", err,
			"driver", sqlDriverName,
			"dsn", dsn,
		)
		return nil, err
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := sqlDB.PingContext(ctx); err != nil {
		_ = sqlDB.Close()
		err = fmt.Errorf("failed to ping database: %w", err)
		logger.Error("Failed to ping database", err,
			"driver", sqlDriverName,
		)
		return nil, err
	}

	sqlDB.SetMaxOpenConns(25)
	sqlDB.SetMaxIdleConns(25)
	sqlDB.SetConnMaxLifetime(5 * time.Minute)

	logger.Info("Database connection established",
		"driver", sqlDriverName,
		"max_open_conns", sqlDB.Stats().OpenConnections,
	)

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

	direction := "up"
	if !up {
		direction = "down"
	}

	logger.Info("Running database migrations",
		"driver", d.Driver,
		"direction", direction,
		"migrations_path", migrationsPath,
	)

	switch d.Driver {
	case "postgres":
		driver, err := pgmigrate.WithInstance(d.DB, &pgmigrate.Config{})
		if err != nil {
			err = fmt.Errorf("failed to create postgres migration driver: %w", err)
			logger.Error("Failed to create postgres migration driver", err,
				"driver", d.Driver,
			)
			return err
		}

		m, err := migrate.NewWithDatabaseInstance(
			sourceURL,
			d.Driver,
			driver,
		)
		if err != nil {
			err = fmt.Errorf("failed to create migrate instance: %w", err)
			logger.Error("Failed to create migrate instance", err,
				"driver", d.Driver,
				"source", sourceURL,
			)
			return err
		}
		// defer m.Close()

		return applyMigrations(m, up)

	case "mysql":
		driver, err := mysqlmigrate.WithInstance(d.DB, &mysqlmigrate.Config{})
		if err != nil {
			err = fmt.Errorf("failed to create mysql migration driver: %w", err)
			logger.Error("Failed to create mysql migration driver", err,
				"driver", d.Driver,
			)
			return err
		}

		m, err := migrate.NewWithDatabaseInstance(
			sourceURL,
			d.Driver,
			driver,
		)
		if err != nil {
			err = fmt.Errorf("failed to create migrate instance: %w", err)
			logger.Error("Failed to create migrate instance", err,
				"driver", d.Driver,
				"source", sourceURL,
			)
			return err
		}
		// defer m.Close()

		return applyMigrations(m, up)

	default:
		err := fmt.Errorf("unsupported migration driver: %s", d.Driver)
		logger.Error("Unsupported migration driver", err,
			"driver", d.Driver,
		)
		return err
	}
}

func applyMigrations(m *migrate.Migrate, up bool) error {
	if up {
		logger.Info("Applying migrations")
		if err := m.Up(); err != nil && !errors.Is(err, migrate.ErrNoChange) {
			err = fmt.Errorf("failed to apply migrations: %w", err)
			logger.Error("Failed to apply migrations", err)
			return err
		}
		logger.Info("Migrations applied successfully")
		return nil
	}

	logger.Info("Rolling back migrations")
	if err := m.Down(); err != nil && !errors.Is(err, migrate.ErrNoChange) {
		err = fmt.Errorf("failed to rollback migrations: %w", err)
		logger.Error("Failed to rollback migrations", err)
		return err
	}
	logger.Info("Migrations rolled back successfully")
	return nil
}
