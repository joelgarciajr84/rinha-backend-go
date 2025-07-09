package httpserver

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
		Addr:         ":9999",
		Handler:      mux,
		ReadTimeout:  5 * time.Second,
		WriteTimeout: 10 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	log.Println("Listening on :9999")
	log.Fatal(srv.ListenAndServe())
}
