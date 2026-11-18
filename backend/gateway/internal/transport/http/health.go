package httpapi

import (
	"net/http"
)

type Readiness interface {
	Ready() bool
	Checks() map[string]bool
}

type healthResponse struct {
	Ready  bool            `json:"ready"`
	Checks map[string]bool `json:"checks"`
}

func Live(w http.ResponseWriter, _ *http.Request) {
	WriteJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func Ready(src Readiness) http.HandlerFunc {
	return func(w http.ResponseWriter, _ *http.Request) {
		ready := src.Ready()
		status := http.StatusOK
		if !ready {
			status = http.StatusServiceUnavailable
		}
		WriteJSON(w, status, healthResponse{Ready: ready, Checks: src.Checks()})
	}
}
