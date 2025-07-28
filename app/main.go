package main

import (
	"fmt"
	"log"
	"net/http"
	"strings"

	"galo/internal/config"
	"galo/internal/infrastructure"
	"galo/internal/interfaces"
	"galo/internal/usecase"
)

func main() {
	settings := config.LoadEnvironmentConfig()

	metricsRepository := infrastructure.NewRedisMetricsRepository(settings.RedisConnectionURL)

	metricsAnalyzer := usecase.NewMetricsAnalyzer(metricsRepository)

	if err := metricsAnalyzer.InitializeSystem(); err != nil {
		log.Printf("Falha ao limpar dados: %v", err)
	}

	transactionProcessor := infrastructure.NewHTTPTransactionProcessor(
		settings.PrimaryProcessorURL,
		settings.FallbackProcessorURL,
		settings.ConcurrencyLimit,
	)

	queueManager := infrastructure.NewChannelQueueManager(settings.QueueCapacity)

	transactionHandler := usecase.NewTransactionHandler(
		settings.PrimaryProcessorURL,
		settings.FallbackProcessorURL,
		metricsRepository,
		transactionProcessor,
		queueManager,
	)

	workerPool := infrastructure.NewWorkerPool(
		settings.WorkerPoolSize,
		transactionHandler,
		queueManager,
	)

	workerPool.StartProcessing()

	transactionController := interfaces.NewTransactionController(
		transactionHandler,
		metricsAnalyzer,
	)

	http.HandleFunc("/payments", transactionController.HandleTransactionSubmission)
	http.HandleFunc("/payments-summary", transactionController.HandleMetricsQuery)

	serverPort := settings.ServerPort
	if !strings.HasPrefix(serverPort, ":") {
		serverPort = ":" + serverPort
	}

	fmt.Printf("GALO DE BRIGA NA PORTA %s\n", serverPort)
	log.Fatal(http.ListenAndServe(serverPort, nil))
}
