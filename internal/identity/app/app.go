package app

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/hzhyvinskyi/distrocommerce/internal/identity/app/config"
	"github.com/hzhyvinskyi/distrocommerce/pkg/database"
	"github.com/hzhyvinskyi/distrocommerce/pkg/graceful"
	"github.com/hzhyvinskyi/distrocommerce/pkg/health"
	"github.com/hzhyvinskyi/distrocommerce/pkg/logger"
	"github.com/hzhyvinskyi/distrocommerce/pkg/migrator"
	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
)

// Run is the Composition Root for identity-sv
//
// Identity is a pure HTTP service - internal services do not call it directly.
// All authentication flows go through BFFs which validate JWTs locally
// using JWKS, then forwards user claims via headers to internal services.
func Run() error {
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}

	log, err := logger.New(cfg.App.Environment, cfg.App.Name, cfg.App.Version)
	if err != nil {
		return fmt.Errorf("init logger: %w", err)
	}
	defer log.Sync() //nolint:errcheck

	log.Info(
		"starting identity-sv",
		zap.String("env", cfg.App.Environment),
	)

	ctx := context.Background()

	db, err := database.NewPostgresPool(ctx, database.PoolConfig{
		DSN:                   cfg.Postgres.DSN(),
		MaxConns:              cfg.Postgres.MaxConns,
		MinConns:              cfg.Postgres.MinConns,
		MaxConnLifetime:       cfg.Postgres.MaxConnLifetime,
		MaxConnLifetimeJitter: cfg.Postgres.MaxConnLifetimeJitter,
		MaxConnIdleTime:       cfg.Postgres.MaxConnIdleTime,
		HealthCheckPeriod:     cfg.Postgres.HealthCheckPeriod,
	})
	if err != nil {
		return fmt.Errorf("init postgres: %w", err)
	}
	defer db.Close()

	if err = migrator.Run(cfg.Postgres.DSN(), "file://migrations/identity", log); err != nil {
		return fmt.Errorf("run migrator: %w", err)
	}

	redisClient := redis.NewClient(&redis.Options{
		Addr:     cfg.Redis.Addr,
		Password: cfg.Redis.Password,
	})
	if err = redisClient.Ping(ctx).Err(); err != nil {
		return fmt.Errorf("ping redis: %w", err)
	}
	defer redisClient.Close() //nolint:errcheck

	httpMux := http.NewServeMux()

	healthChecker := health.New(cfg.App.Name, cfg.App.Version, db, redisClient)
	httpMux.HandleFunc("/healthz", healthChecker.LiveHandler())
	httpMux.HandleFunc("/readyz", healthChecker.ReadyHandler())

	httpSrv := &http.Server{
		Addr:              fmt.Sprintf(":%d", cfg.HTTP.Port),
		Handler:           httpMux,
		ReadTimeout:       10 * time.Second,
		ReadHeaderTimeout: 5 * time.Second,
		WriteTimeout:      10 * time.Second,
		MaxHeaderBytes:    1 << 20,
	}

	// Graceful Shutdown
	runner := graceful.New(log, cfg.HTTP.ShutdownTimeout)
	if err = runner.NotifyOnShutdown(healthChecker); err != nil {
		return err
	}
	if err = runner.Add(1, graceful.HTTPShutdownHook(httpSrv), "http-server"); err != nil {
		return err
	}

	go func() {
		log.Info(
			"identity-sv HTTP listening",
			zap.String("addr", httpSrv.Addr),
		)
		if err = httpSrv.ListenAndServe(); err != nil && !errors.Is(http.ErrServerClosed, err) {
			log.Error("http server error", zap.Error(err))
		}
	}()

	return runner.Run(ctx)
}
