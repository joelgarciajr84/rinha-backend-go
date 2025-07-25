package main

import (
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"rinha-go-joel/src/database"
	"rinha-go-joel/src/gateway"
	"rinha-go-joel/src/processor"
	"rinha-go-joel/src/service"
	"rinha-go-joel/src/storage"
	"rinha-go-joel/src/transport"
	"rinha-go-joel/src/utils"
	"runtime"
	"syscall"
	"time"
)

func main() {
	envLoader := utils.NewEnvLoader()
	envLoader.LoadEnvironment()

	runtime.GOMAXPROCS(runtime.NumCPU())

	dbConfig := database.NewDatabaseConfig()
	defer dbConfig.Pool.Close()

	processorGateway, err := gateway.NewProcessorGateway()
	if err != nil {
		log.Fatal("Failed to create processor gateway:", err)
	}
	transactionRepo := storage.NewTransactionRepository(dbConfig.Pool)
	transactionSvc := service.NewTransactionService(processorGateway, transactionRepo)

	transactionProcessor := processor.NewTransactionProcessor(transactionSvc)
	transactionProcessor.Start()
	defer transactionProcessor.Stop()

	httpHandler := transport.NewHTTPHandler(transactionProcessor, transactionSvc)

	http.HandleFunc("/payments", httpHandler.HandleTransactions)
	http.HandleFunc("/payments-summary", httpHandler.HandleTransactionSummary)
	http.HandleFunc("/health", httpHandler.HandleHealthCheck)

	port := utils.GetString("PORT", "8080")
	server := &http.Server{
		Addr:         fmt.Sprintf(":%s", port),
		Handler:      nil,
		ReadTimeout:  100 * time.Millisecond,
		WriteTimeout: 500 * time.Millisecond,
		IdleTimeout:  120 * time.Second,
	}

	go func() {
		log.Printf("Server starting on port %s", port)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("Server failed to start: %v", err)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Println("Server shutting down gracefully...")
}
