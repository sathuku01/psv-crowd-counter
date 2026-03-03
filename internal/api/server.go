package api

import (
	"encoding/json"
	"net/http"

	"psv-crowd-counter/internal/gps"
	"psv-crowd-counter/internal/service"
	"psv-crowd-counter/internal/storage"
)

func NewServer(store storage.Store, gps gps.GPS, proc *service.Processor) *http.ServeMux {
	mux := http.NewServeMux()
	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]interface{}{"status": "ok", "speed_kph": gps.CurrentSpeedKPH(), "processor": proc.Status()})
	})

	mux.HandleFunc("/reports", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		reps, err := store.List()
		if err != nil {
			http.Error(w, "internal error", http.StatusInternalServerError)
			return
		}
		_ = json.NewEncoder(w).Encode(reps)
	})

	return mux
}
