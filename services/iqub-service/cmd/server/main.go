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

func main() {
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	repo := memory.NewIqubRepository()
	iqubService := application.NewIqubService(repo)
	contributionService := application.NewContributionService(repo)
	handler := httpapi.NewHandler(iqubService, contributionService)

	server := &http.Server{
		Addr:              ":" + port,
		Handler:           httpapi.NewRouter(handler),
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       10 * time.Second,
		WriteTimeout:      10 * time.Second,
		IdleTimeout:       60 * time.Second,
	}

	log.Printf("iqub-service listening on :%s", port)
	log.Fatal(server.ListenAndServe())
}
