package main

import (
	"bytes"
	"encoding/json"
	"log"
	"net/http"
	"os"
	"sync"
	"time"
)

type Payment struct {
	CorrelationID string  `json:"correlationId"`
	Amount        float64 `json:"amount"`
	RequestedAt   string  `json:"requestedAt,omitempty"`
}

type ProcessorStats struct {
	TotalRequests int     `json:"totalRequests"`
	TotalAmount   float64 `json:"totalAmount"`
}

type SummaryResponse struct {
	Default  ProcessorStats `json:"default"`
	Fallback ProcessorStats `json:"fallback"`
}

type Health struct {
	Failing         bool `json:"failing"`
	MinResponseTime int  `json:"minResponseTime"`
}

var (
	defaultURL  = os.Getenv("PAYMENT_PROCESSOR_URL_DEFAULT")
	fallbackURL = os.Getenv("PAYMENT_PROCESSOR_URL_FALLBACK")
	mutex       sync.Mutex
	stats       = map[string]*ProcessorStats{
		"default":  {0, 0},
		"fallback": {0, 0},
	}
	healthCache       = map[string]Health{}
	healthLastCheck   = map[string]time.Time{}
	healthCheckMutex  sync.Mutex
	paymentQueue      = make(chan Payment, 10000)
	processedPayments = make([]Payment, 0, 10000) // Armazena pagamentos processados
	processedIDs      = make(map[string]struct{}) // Para garantir unicidade
)

func main() {
	go worker()

	http.HandleFunc("/payments", handlePayment)
	http.HandleFunc("/payments-summary", handleSummary)

	log.Println("Listening on :9999")
	log.Fatal(http.ListenAndServe(":9999", nil))
}

func handlePayment(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	var p Payment
	if err := json.NewDecoder(r.Body).Decode(&p); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	// Validação correlationId (UUID) e amount
	if p.CorrelationID == "" || !isValidUUID(p.CorrelationID) {
		w.WriteHeader(http.StatusBadRequest)
		return
	}
	if p.Amount == 0 {
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	mutex.Lock()
	if _, exists := processedIDs[p.CorrelationID]; exists {
		mutex.Unlock()
		w.WriteHeader(http.StatusConflict)
		return
	}
	processedIDs[p.CorrelationID] = struct{}{}
	mutex.Unlock()

	p.RequestedAt = time.Now().UTC().Format(time.RFC3339Nano)
	paymentQueue <- p

	w.WriteHeader(http.StatusAccepted)
}

func handleSummary(w http.ResponseWriter, r *http.Request) {
	fromStr := r.URL.Query().Get("from")
	toStr := r.URL.Query().Get("to")
	var from, to time.Time
	var err error
	if fromStr != "" {
		from, err = time.Parse(time.RFC3339, fromStr)
		if err != nil {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
	}
	if toStr != "" {
		to, err = time.Parse(time.RFC3339, toStr)
		if err != nil {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
	}

	mutex.Lock()
	defer mutex.Unlock()
	// Filtra pagamentos processados
	summary := map[string]*ProcessorStats{
		"default":  {0, 0},
		"fallback": {0, 0},
	}
	for _, p := range processedPayments {
		t, err := time.Parse(time.RFC3339Nano, p.RequestedAt)
		if err != nil {
			continue
		}
		if (fromStr == "" || !t.Before(from)) && (toStr == "" || !t.After(to)) {
			key := "default"
			if p.CorrelationID != "" && p.CorrelationID[0] == 'F' { // fallback marker (ajuste se necessário)
				key = "fallback"
			}
			if p.Amount > 0 {
				summary[key].TotalRequests++
				summary[key].TotalAmount += p.Amount
			}
		}
	}
	resp := SummaryResponse{
		Default:  *summary["default"],
		Fallback: *summary["fallback"],
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

func worker() {
	for p := range paymentQueue {
		url := pickProcessor()
		sendPayment(url, p)
	}
}

func sendPayment(url string, p Payment) {
	body, _ := json.Marshal(p)
	resp, err := http.Post(url+"/payments", "application/json", bytes.NewReader(body))

	if err != nil || resp.StatusCode >= 500 {
		if url == defaultURL {
			log.Println("Default falhou, tentando fallback")
			sendPayment(fallbackURL, p)
			return
		}
		log.Println("Fallback falhou também")
		return
	}

	typeKey := "default"
	if url == fallbackURL {
		typeKey = "fallback"
	}

	mutex.Lock()
	stats[typeKey].TotalRequests++
	stats[typeKey].TotalAmount += p.Amount
	// Armazena pagamento processado
	processedPayments = append(processedPayments, p)
	mutex.Unlock()
}

// isValidUUID faz uma checagem simples de UUID v4
func isValidUUID(u string) bool {
	if len(u) != 36 {
		return false
	}
	for i, c := range u {
		switch i {
		case 8, 13, 18, 23:
			if c != '-' {
				return false
			}
		default:
			if c == '-' {
				return false
			}
		}
	}
	return true
}

func pickProcessor() string {
	defaultHealth := getHealth(defaultURL)
	if !defaultHealth.Failing {
		return defaultURL
	}
	return fallbackURL
}

func getHealth(url string) Health {
	healthCheckMutex.Lock()
	defer healthCheckMutex.Unlock()

	last, ok := healthLastCheck[url]
	if ok && time.Since(last) < 5*time.Second {
		return healthCache[url]
	}

	resp, err := http.Get(url + "/payments/service-health")
	if err != nil || resp.StatusCode != 200 {
		healthCache[url] = Health{Failing: true, MinResponseTime: 9999}
	} else {
		var h Health
		json.NewDecoder(resp.Body).Decode(&h)
		healthCache[url] = h
	}
	healthLastCheck[url] = time.Now()
	return healthCache[url]
}
