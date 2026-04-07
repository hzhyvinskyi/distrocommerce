package app

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/hzhyvinskyi/distrocommerce/internal/identity/app/config"
	"github.com/hzhyvinskyi/distrocommerce/pkg/graceful"
	"github.com/hzhyvinskyi/distrocommerce/pkg/health"
	"github.com/hzhyvinskyi/distrocommerce/pkg/logger"
	"go.uber.org/zap"
)

// Run is the Composition Root for identity-sv
//
// Identity is a pure HTTP service - internal services do not call it directly.
// All authentication flows go through api-gateway which validates JWTs locally
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

	httpMux := http.NewServeMux()

	healthChecker := health.New(cfg.App.Name, cfg.App.Version, nil, nil)
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
