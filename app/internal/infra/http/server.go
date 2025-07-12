package http

import (
	"log"
	"net/http"
	"rinha/internal/handlers"
	"time"
)

func StartServer(handler *handlers.PaymentHandler) {
	mux := http.NewServeMux()
	mux.HandleFunc("/payments", handler.HandlePayment)
	mux.HandleFunc("/payments-summary", handler.HandleSummary)

	srv := &http.Server{
		Addr:              ":9999",
		Handler:           mux,
		ReadTimeout:       2 * time.Second,
		WriteTimeout:      3 * time.Second,
		ReadHeaderTimeout: 1 * time.Second,
		IdleTimeout:       30 * time.Second,
		MaxHeaderBytes:    1 << 16,
	}

	log.Println("Servidor rodando na porta :9999")
	log.Fatal(srv.ListenAndServe())
}
