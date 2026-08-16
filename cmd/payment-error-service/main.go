package main

import (
	"encoding/json"
	"log"
	"net/http"
	"os"

	paymenterrors "github.com/example/fintech-error-ledger"
)

func main() {
	apiKey := os.Getenv("INFRAI_API_KEY")
	if apiKey == "" {
		log.Fatal("INFRAI_API_KEY is required")
	}
	processor := paymenterrors.Processor{Errors: paymenterrors.NewErrorClient(apiKey), ReviewAtScore: 80}
	mux := http.NewServeMux()
	mux.HandleFunc("POST /payment-events", func(w http.ResponseWriter, r *http.Request) {
		var event paymenterrors.PaymentEvent
		if err := json.NewDecoder(r.Body).Decode(&event); err != nil {
			http.Error(w, "invalid payment event", http.StatusBadRequest)
			return
		}
		note, err := processor.Process(r.Context(), event)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadGateway)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(note)
	})
	log.Println("payment error service listening on :8080")
	log.Fatal(http.ListenAndServe(":8080", mux))
}
