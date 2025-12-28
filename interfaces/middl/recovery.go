package middl

import (
	"encoding/json"
	"log"
	"net/http"
	"runtime/debug"
	"time"
)

type ErrorResponse struct {
	Status    string `json:"status"`
	Message   string `json:"message"`
	Timestamp string `json:"timestamp"`
}

// Recovery middleware PRO
func Recovery(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {

		defer func() {
			if rec := recover(); rec != nil {

				// Log interne (console)
				log.Printf(
					"[PANIC] %v\nSTACKTRACE:\n%s",
					rec,
					debug.Stack(),
				)

				// Réponse API JSON
				resp := ErrorResponse{
					Status:    "error",
					Message:   "Internal server error",
					Timestamp: time.Now().Format(time.RFC3339),
				}

				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusInternalServerError)
				_ = json.NewEncoder(w).Encode(resp)
			}
		}()

		next.ServeHTTP(w, r)
	})
}
