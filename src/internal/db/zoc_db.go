package db

import (
	"context"
	"fmt"
	"sync"
	"time"

	"zoc/src/internal/logger"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

var (
	zocPool *pgxpool.Pool
	zocOnce sync.Once
)

// InitZocDB connects to the Zoc database. Tables (folders, documents,
// zoc_document_versions) are created manually ahead of time — this does
// not run any schema migrations, only connects and pings.
func InitZocDB(ctx context.Context, dbURL string) (*pgxpool.Pool, error) {
	if dbURL == "" {
		logger.LogDB("DATABASE_URL not set; skipping DB initialization")
		return nil, nil
	}

	var err error
	zocOnce.Do(func() {
		logger.LogDB("Initializing Zoc database pool...")

		config, parseErr := pgxpool.ParseConfig(dbURL)
		if parseErr != nil {
			err = fmt.Errorf("failed to parse Zoc database URL: %w", parseErr)
			return
		}
		config.ConnConfig.DefaultQueryExecMode = pgx.QueryExecModeSimpleProtocol

		retryErr := retryWithExponentialBackoff(ctx, 5, 1*time.Second, 30*time.Second, func() error {
			var connErr error
			zocPool, connErr = pgxpool.NewWithConfig(ctx, config)
			if connErr != nil {
				return fmt.Errorf("failed to connect to Zoc database: %w", connErr)
			}
			if pingErr := zocPool.Ping(ctx); pingErr != nil {
				zocPool.Close()
				zocPool = nil
				return fmt.Errorf("failed to ping Zoc database: %w", pingErr)
			}
			return nil
		}, func(format string, args ...any) {
			logger.LogDB(format, args...)
		})

		if retryErr != nil {
			err = fmt.Errorf("Zoc database initialization failed after retries: %w", retryErr)
			return
		}
		logger.LogDB("Zoc DB connection pool initialized successfully.")
	})

	return zocPool, err
}

func GetZocPoolOrNil() *pgxpool.Pool {
	return zocPool
}

func ZocPoolReady() bool {
	return zocPool != nil
}

func CloseZocDB() {
	if zocPool != nil {
		logger.LogDB("Closing Zoc database connection pool.")
		zocPool.Close()
	}
}
