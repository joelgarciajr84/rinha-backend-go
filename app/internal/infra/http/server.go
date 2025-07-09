package httpserver

import (
	"log"
	"net/http"
	"rinha/internal/handlers"
)

func StartServer(handler *handlers.PaymentHandler) {
	http.HandleFunc("/payments", handler.HandlePayment)
	http.HandleFunc("/payments-summary", handler.HandleSummary)

	log.Println("Listening on :9999")
	log.Fatal(http.ListenAndServe(":9999", nil))
}
