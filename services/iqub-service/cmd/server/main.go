package main

import (
	"context"
	"errors"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/selamawitsh/iqub-platform/services/iqub-service/internal/application"
	"github.com/selamawitsh/iqub-platform/services/iqub-service/internal/infrastructure/postgres"
	"github.com/selamawitsh/iqub-platform/services/iqub-service/internal/presentation/httpapi"
)

// main is the composition root: the only place that knows every layer.
// It builds the concrete pieces from the inside out and connects them.
func main() {
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	// ctx is cancelled on Ctrl+C or SIGTERM (what `docker stop` sends).
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	db, err := postgres.Open(ctx, os.Getenv("DATABASE_URL"))
	if err != nil {
		log.Fatalf("database startup failed: %v", err)
	}
	defer db.Close()

	repo := postgres.NewIqubRepository(db)                          // infrastructure
	iqubService := application.NewIqubService(repo)                 // application
	contributionService := application.NewContributionService(repo) // application
	handler := httpapi.NewHandler(iqubService, contributionService) // presentation

	server := &http.Server{
		Addr:              ":" + port,
		Handler:           httpapi.NewRouter(handler),
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       10 * time.Second,
		WriteTimeout:      10 * time.Second,
		IdleTimeout:       60 * time.Second,
	}

	serverErr := make(chan error, 1)
	go func() {
		log.Printf("iqub-service listening on :%s", port)
		serverErr <- server.ListenAndServe()
	}()

	select {
	case err := <-serverErr:
		// The server stopped by itself (for example the port is taken).
		if !errors.Is(err, http.ErrServerClosed) {
			log.Fatalf("HTTP server failed: %v", err)
		}
	case <-ctx.Done():
		// Graceful shutdown (SRS NFR-R3): stop taking new requests and wait
		// for running ones to finish. main must wait for Shutdown to return,
		// otherwise the process exits and cuts those requests off.
		log.Println("shutting down")
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if err := server.Shutdown(shutdownCtx); err != nil {
			log.Printf("shutdown error: %v", err)
		}
	}
	// Deferred calls run now: the database connections close last.
}
