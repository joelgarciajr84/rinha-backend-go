package main

import (
	"bytes"
	"encoding/json"
	"net/http"
	"os"

	"rinha/internal/core/domain"
	"rinha/internal/core/service"
	"rinha/internal/handlers"
	"rinha/internal/infra/health"
	httpserver "rinha/internal/infra/http"
	"rinha/internal/infra/storage"
)

func main() {
	defaultURL := os.Getenv("PAYMENT_PROCESSOR_URL_DEFAULT")
	fallbackURL := os.Getenv("PAYMENT_PROCESSOR_URL_FALLBACK")

	memStorage := storage.NewMemoryStorage()
	healthChecker := health.NewChecker()

	sender := func(url string, p domain.Payment) error {
		body, _ := json.Marshal(p)
		resp, err := http.Post(url+"/payments", "application/json", bytes.NewReader(body))
		if err != nil || resp.StatusCode >= 500 {
			return err
		}
		processor := "default"
		if url == fallbackURL {
			processor = "fallback"
		}
		memStorage.SavePayment(p, processor)
		return nil
	}

	paymentService := service.NewPaymentService(memStorage, healthChecker, sender, defaultURL, fallbackURL)
	handler := handlers.NewHandler(*paymentService, memStorage)

	httpserver.StartServer(handler)
}
