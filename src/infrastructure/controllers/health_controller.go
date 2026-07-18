package controllers

import (
	"encoding/json"
	"net/http"
)

// HealthController expone el estado del servicio para checks de
// liveness/readiness (Docker, orquestador, balanceador, etc.).
type HealthController struct{}

func NewHealthController() *HealthController {
	return &HealthController{}
}

func (h *HealthController) Health(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(map[string]string{
		"status":  "ok",
		"service": "ingesta-iot",
	})
}
