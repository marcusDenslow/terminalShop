package handlers

import (
	"net/http"
	"strconv"
	"terminalShop/api/middleware"
	"terminalShop/pkg/database"
	"terminalShop/pkg/models"
	"terminalShop/pkg/utils"

	"github.com/go-chi/chi/v5"
)

type SSHKeyHandler struct{}

func NewSSHKeyHandler() *SSHKeyHandler {
	return &SSHKeyHandler{}
}

// GetSSHKeys returns all ssh keys for the authenticated user.
func (h *SSHKeyHandler) GetSSHKeys(w http.ResponseWriter, r *http.Request) {
	db := database.GetDB()
	userID := middleware.UserIDFromContext(r.Context())

	var keys []models.SSHKey
	db.Where("user_id = ?", userID).Order("created_at ASC").Find(&keys)

	utils.RespondSuccess(w, http.StatusOK, map[string]interface{}{
		"ssh_keys": keys,
	})
}

// DeleteSSHKeys removes one of the users ssh keys by ID
func (h *SSHKeyHandler) DeleteSSHKeys(w http.ResponseWriter, r *http.Request) {
	db := database.GetDB()
	userID := middleware.UserIDFromContext(r.Context())

	idStr := chi.URLParam(r, "id")
	id, err := strconv.ParseUint(idStr, 10, 32)
	if err != nil {
		utils.RespondError(w, http.StatusBadRequest, "INVALID_ID", "invalid ssh key id", nil)
		return
	}

	var key models.SSHKey
	if err := db.Where("id = ? AND user_id = ?", id, userID).First(&key).Error; err != nil {
		utils.RespondError(w, http.StatusNotFound, "SSH_KEY_NOT_FOUND", "ssh key not found", nil)
		return
	}
	if err := db.Delete(&key).Error; err != nil {
		utils.RespondError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "failed to delete ssh key", nil)
		return
	}

	utils.RespondSuccess(w, http.StatusOK, map[string]interface{}{
		"message": "ssh key deleted",
	})
}
