package main

import (
	"bytes"
	"encoding/json"
	"log"
	"net/http"
	"os"
	"runtime"
	"time"

	"rinha/internal/core/domain"
	"rinha/internal/core/service"
	"rinha/internal/handlers"
	"rinha/internal/infra/health"
	httpserver "rinha/internal/infra/http"
	"rinha/internal/infra/queue"
	"rinha/internal/infra/storage"
)

func main() {
	defaultURL := os.Getenv("PAYMENT_PROCESSOR_URL_DEFAULT")
	fallbackURL := os.Getenv("PAYMENT_PROCESSOR_URL_FALLBACK")

	redisAddr := os.Getenv("REDIS_ADDR")
	if redisAddr == "" {
		redisAddr = "redis:6379"
	}
	redisStorage := storage.NewRedisStorage(redisAddr)
	healthChecker := health.NewChecker()

	client := &http.Client{
		Timeout: 2 * time.Second,
		Transport: &http.Transport{
			MaxIdleConns:        1000,
			MaxIdleConnsPerHost: 1000,
		},
	}

	q := queue.NewPaymentQueue(10000)

	workerCount := 32
	for i := 0; i < workerCount; i++ {
		go func() {
			for task := range q.Dequeue() {
				if !redisStorage.TryMarkProcessing(task.Payment.CorrelationID) {
					continue
				}
				tryCount := 0
				processed := false
				for tryCount < 2 {
					url := defaultURL
					processor := "default"
					if tryCount == 1 {
						url = fallbackURL
						processor = "fallback"
					}
					body, _ := json.Marshal(task.Payment)
					resp, err := client.Post(url+"/payments", "application/json", bytes.NewReader(body))
					if err == nil && resp.StatusCode < 500 {
						redisStorage.SavePayment(task.Payment, processor)
						processed = true
						break
					}
					tryCount++
				}
				if !processed {
					redisStorage.UnmarkProcessing(task.Payment.CorrelationID)
				}
			}
		}()
	}

	sender := func(url string, p domain.Payment) error {
		q.Enqueue(queue.PaymentTask{Payment: p, Processor: "default"})
		return nil
	}

	paymentService := service.NewPaymentService(redisStorage, healthChecker, sender, defaultURL, fallbackURL)
	handler := handlers.NewHandler(*paymentService, redisStorage)

	// Monitoramento
	go func() {
		for {
			var m runtime.MemStats
			runtime.ReadMemStats(&m)
			log.Printf("[MONITOR] Mem: %.2fMB | Goroutines: %d", float64(m.Alloc)/1024/1024, runtime.NumGoroutine())
			time.Sleep(10 * time.Second)
		}
	}()

	httpserver.StartServer(handler)
}
