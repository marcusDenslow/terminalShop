package handlers

import (
	"net/http"
	"time"

	"terminalShop/pkg/utils"
)

// HealthHandler handles health check requests
type HealthHandler struct {
	version string
}

// NewHealthHandler creates a new health handler
func NewHealthHandler(version string) *HealthHandler {
	return &HealthHandler{
		version: version,
	}
}

// GetHealth returns the health status of the API
func (h *HealthHandler) GetHealth(w http.ResponseWriter, r *http.Request) {
	data := utils.HealthData{
		Status:    "ok",
		Timestamp: time.Now(),
		Version:   h.version,
	}

	utils.RespondSuccess(w, http.StatusOK, data)
}

// Ping returns a simple pong response for testing
func (h *HealthHandler) Ping(w http.ResponseWriter, r *http.Request) {
	utils.RespondSuccess(w, http.StatusOK, map[string]string{
		"message": "pong",
	})
}
