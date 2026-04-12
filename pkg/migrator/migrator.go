package migrator

import (
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/postgres"
	_ "github.com/golang-migrate/migrate/v4/source/file"
	"go.uber.org/zap"
)

func Run(dsn, migrationsPath string, logger *zap.Logger) error {
	m, err := migrate.New(migrationsPath, dsn)
	if err != nil {
		return fmt.Errorf("init migrator (%s): %w", migrationsPath, err)
	}
	defer func() {
		srcErr, dbErr := m.Close()
		if srcErr != nil || dbErr != nil {
			logger.Warn("migrate close error",
				zap.Error(srcErr),
				zap.Error(dbErr),
			)
		}
	}()

	if err := m.Up(); err != nil {
		if errors.Is(err, migrate.ErrNoChange) {
			logger.Info(
				"migrations: no changes",
				zap.String("path", migrationsPath),
			)

			return nil
		}

		version, dirty, vErr := m.Version()
		if vErr != nil && !errors.Is(vErr, migrate.ErrNilVersion) {
			return fmt.Errorf("get version: %w", vErr)
		}

		if dirty {
			return fmt.Errorf(
				"database is dirty at version %d: fix with `migrate force %d`",
				version,
				version,
			)
		}

		return fmt.Errorf("run migrations (%s): %w", migrationsPath, err)
	}

	version, _, vErr := m.Version()
	if vErr != nil && !errors.Is(vErr, migrate.ErrNilVersion) {
		logger.Warn("cannot get migration version", zap.Error(vErr))
	} else {
		logger.Info("migrations applied",
			zap.String("path", migrationsPath),
			zap.Uint("version", version),
		)
	}

	return nil
}

// Down rolls back all migrations. For use in tests only — never call in production.
func Down(dsn, migrationsPath string, logger *zap.Logger) error {
	env := strings.ToLower(os.Getenv("APP_ENV"))
	if env == "production" || env == "prod" {
		return errors.New("down migrations are disabled in production")
	}

	m, err := migrate.New(migrationsPath, dsn)
	if err != nil {
		return fmt.Errorf("init migrator: %w", err)
	}
	defer func() {
		srcErr, dbErr := m.Close()
		if srcErr != nil || dbErr != nil {
			logger.Warn("migrate close error",
				zap.Error(srcErr),
				zap.Error(dbErr),
			)
		}
	}()

	return m.Down()
}
