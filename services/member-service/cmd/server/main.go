package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/selamawitsh/iqub-platform/services/member-service/internal/application"
	"github.com/selamawitsh/iqub-platform/services/member-service/internal/config"
	"github.com/selamawitsh/iqub-platform/services/member-service/internal/infrastructure/clock"
	"github.com/selamawitsh/iqub-platform/services/member-service/internal/infrastructure/id"
	"github.com/selamawitsh/iqub-platform/services/member-service/internal/infrastructure/messaging"
	"github.com/selamawitsh/iqub-platform/services/member-service/internal/infrastructure/postgres"
	"github.com/selamawitsh/iqub-platform/services/member-service/internal/infrastructure/security"
	"github.com/selamawitsh/iqub-platform/services/member-service/internal/presentation/httpapi"
)

func main() {
	if err := run(); err != nil {
		slog.Error("member-service stopped with error", slog.String("error", err.Error()))
		os.Exit(1)
	}
}

func run() error {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))

	cfg, err := config.Load()
	if err != nil {
		return err
	}

	// Cancelled on SIGINT/SIGTERM, which starts the graceful shutdown (NFR-R3).
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	pool, err := postgres.NewPool(ctx, cfg.DatabaseURL)
	if err != nil {
		return err
	}
	defer pool.Close()

	if err := postgres.Migrate(ctx, pool); err != nil {
		return err
	}

	publisher := messaging.NewRabbitMQPublisher(cfg.RabbitMQURL, logger)
	defer publisher.Close()

	// Composition root: the only place that knows every concrete type.
	service := application.NewMemberService(
		postgres.NewMemberRepository(pool),
		security.NewBcryptHasher(security.DefaultBcryptCost),
		security.NewJWTIssuer(cfg.JWTSecret, cfg.JWTTTL),
		id.UUIDGenerator{},
		clock.SystemClock{},
		publisher,
		logger,
	)

	handler := httpapi.NewHandler(service, pool, logger)
	srv := &http.Server{
		Addr:              ":" + cfg.Port,
		Handler:           httpapi.NewRouter(handler, logger),
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       10 * time.Second,
		WriteTimeout:      10 * time.Second,
		IdleTimeout:       60 * time.Second,
	}

	errCh := make(chan error, 1)
	go func() {
		logger.Info("member-service listening", slog.String("addr", srv.Addr))
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			errCh <- err
		}
	}()

	select {
	case err := <-errCh:
		return err
	case <-ctx.Done():
		logger.Info("shutting down")
	}

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	return srv.Shutdown(shutdownCtx)
}
