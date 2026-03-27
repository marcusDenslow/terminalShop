package handlers

import (
	"encoding/json"
	"net/http"

	"terminalShop/pkg/auth"
	"terminalShop/pkg/database"
	"terminalShop/pkg/models"
	"terminalShop/pkg/utils"

	"gorm.io/gorm"
)

// AuthHandler handles authentication-related requests
type AuthHandler struct {
	jwtManager         *auth.JWTManager
	authFingerprintKey string
}

// NewAuthHandler creates a new auth handler
func NewAuthHandler(jwtManager *auth.JWTManager, authFingerprintKey string) *AuthHandler {
	return &AuthHandler{
		jwtManager:         jwtManager,
		authFingerprintKey: authFingerprintKey,
	}
}

// Type to represent a request to exchange SSH fingerprint for JWT
type TokenRequest struct {
	Fingerprint  string `json:"fingerprint"`
	SSHPublicKey string `json:"ssh_public_key"`
	ClientSecret string `json:"client_secret"`
}

// Type to represent the JWT token response
type TokenResponse struct {
	AccessToken string            `json:"access_token"`
	TokenType   string            `json:"token_type"`
	ExpiresIn   int               `json:"expires_in"`
	User        models.PublicUser `json:"user"`
}

func (h *AuthHandler) GetToken(w http.ResponseWriter, r *http.Request) {
	db := database.GetDB()

	var req TokenRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		utils.RespondError(w, http.StatusBadRequest, "INVALID_JSON", "invalid request body", nil)
		return
	}

	// validate shared secret
	if req.ClientSecret != h.authFingerprintKey {
		utils.RespondError(w, http.StatusUnauthorized, "INVALID_SECRET", "invalid client secret", nil)
		return
	}

	// Return if no fingerprint is found
	if req.Fingerprint == "" {
		utils.RespondError(w, http.StatusBadRequest, "MISSING_FINGERPRINT", "fingerprint is required", nil)
		return
	}

	// look up user by his fingies
	var user models.User
	err := db.Where("ssh_key_fingerprint = ?", req.Fingerprint).First(&user).Error
	if err == gorm.ErrRecordNotFound {
		if req.SSHPublicKey == "" {
			utils.RespondError(w, http.StatusUnauthorized, "USER_NOT_FOUND", "no user found for this fingerprint", nil)
			return
		}
		user = models.User{
			SSHKeyFingerprint: req.Fingerprint,
			SSHPublicKey: req.SSHPublicKey,
		}
		if err := db.Create(&user).Error; err != nil {
			utils.RespondError(w, http.StatusInternalServerError, "DATABASE_ERROR", "failed to create user", nil)
			return
		}
	} else if err != nil {
		utils.RespondError(w, http.StatusInternalServerError, "DATABASE_ERROR", "database error", nil)
		return
	}

	token, err := h.jwtManager.GenerateToken(user.ID, user.Email, user.Name)
	if err != nil {
		utils.RespondError(w, http.StatusInternalServerError, "TOKEN_ERROR", "failed to generate token", nil)
		return
	}

	utils.RespondSuccess(w, http.StatusOK, map[string]interface{}{
		"access_token": token,
		"token_type":   "Bearer",
		"expires_in":   1800, // 30 minutes in seconds
		"user": user.ToPublic(),
	})
}

// RegisterRequest represents a user registration request
type RegisterRequest struct {
	SSHPublicKey      string `json:"ssh_public_key"`
	SSHKeyFingerprint string `json:"ssh_key_fingerprint"`
	Name              string `json:"name,omitempty"`  // Optional
	Email             string `json:"email,omitempty"` // Optional
}

// RegisterWithSSHKey registers a new user with their SSH public key
// No username required - following terminal.shop pattern
func (h *AuthHandler) RegisterWithSSHKey(w http.ResponseWriter, r *http.Request) {
	db := database.GetDB()

	var req RegisterRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		utils.RespondError(w, http.StatusBadRequest, "INVALID_JSON", "invalid request body", nil)
		return
	}

	// Validate required fields
	errors := map[string]string{}
	if req.SSHPublicKey == "" {
		errors["ssh_public_key"] = "SSH public key is required"
	}

	if req.SSHKeyFingerprint == "" {
		errors["ssh_key_fingerprint"] = "SSH key fingerprint is required"
	}

	if len(errors) > 0 {
		utils.RespondError(w, http.StatusBadRequest, "VALIDATION_ERROR", "validation failed", map[string]interface{}{
			"errors": errors,
		})
		return
	}

	// Check if SSH key is already registered
	var existing models.User
	err := db.Where("ssh_key_fingerprint = ?", req.SSHKeyFingerprint).First(&existing).Error
	if err == nil {
		utils.RespondError(w, http.StatusConflict, "SSH_KEY_REGISTERED", "SSH key already registered", nil)
		return
	}

	// Create new user (no username required!)
	user := models.User{
		SSHPublicKey:      req.SSHPublicKey,
		SSHKeyFingerprint: req.SSHKeyFingerprint,
		Name:              req.Name,
		Email:             req.Email,
	}

	if err := db.Create(&user).Error; err != nil {
		utils.RespondError(w, http.StatusInternalServerError, "DATABASE_ERROR", "failed to create user", nil)
		return
	}

	utils.RespondSuccess(w, http.StatusCreated, map[string]interface{}{
		"user": user.ToPublic(),
	})
}

// GetUserBySSHKey retrieves a user by their SSH key fingerprint
func (h *AuthHandler) GetUserBySSHKey(w http.ResponseWriter, r *http.Request) {
	db := database.GetDB()

	fingerprint := r.URL.Query().Get("fingerprint")
	if fingerprint == "" {
		utils.RespondError(w, http.StatusBadRequest, "MISSING_FINGERPRINT", "fingerprint query parameter is required", nil)
		return
	}

	var user models.User
	err := db.Where("ssh_key_fingerprint = ?", fingerprint).First(&user).Error
	if err != nil {
		utils.RespondError(w, http.StatusNotFound, "USER_NOT_FOUND", "user not found", nil)
		return
	}

	utils.RespondSuccess(w, http.StatusOK, map[string]interface{}{
		"user": user.ToPublic(),
	})
}
