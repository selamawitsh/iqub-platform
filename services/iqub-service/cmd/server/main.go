package main

import (
	"log"
	"net/http"
	"os"
	"time"

	"github.com/selamawitsh/iqub-platform/services/iqub-service/internal/application"
	"github.com/selamawitsh/iqub-platform/services/iqub-service/internal/infrastructure/memory"
	"github.com/selamawitsh/iqub-platform/services/iqub-service/internal/presentation/httpapi"
)

// main is the composition root: the only place that knows every layer.
// It builds the concrete pieces from the inside out and connects them.
func main() {
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	repo := memory.NewIqubRepository()               // infrastructure
	service := application.NewIqubService(repo)      // application
	handler := httpapi.NewHandler(service)           // presentation

	srv := &http.Server{
		Addr:              ":" + port,
		Handler:           httpapi.NewRouter(handler),
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       10 * time.Second,
		WriteTimeout:      10 * time.Second,
		IdleTimeout:       60 * time.Second,
	}

	log.Printf("iqub-service listening on :%s", port)
	log.Fatal(srv.ListenAndServe())
}
