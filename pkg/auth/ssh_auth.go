package auth

import (
	"fmt"

	gossh "golang.org/x/crypto/ssh"
	"gorm.io/gorm"
	"terminalShop/pkg/models"
)

// SSHAuthService handles SSH key-based authentication
type SSHAuthService struct {
	db         *gorm.DB
	jwtManager *JWTManager
}

// NewSSHAuthService creates a new SSH auth service
func NewSSHAuthService(db *gorm.DB, jwtManager *JWTManager) *SSHAuthService {
	return &SSHAuthService{
		db:         db,
		jwtManager: jwtManager,
	}
}

// AuthenticateSSHKey authenticates a user by their SSH public key
// Auto-creates user on first connect (following terminal.shop pattern)
// Returns the user and a JWT token
func (s *SSHAuthService) AuthenticateSSHKey(publicKey gossh.PublicKey) (*models.User, string, error) {
	fingerprint := GetSSHKeyFingerprint(publicKey)
	sshKey := FormatSSHPublicKey(publicKey)

	var user models.User
	err := s.db.Where("ssh_key_fingerprint = ?", fingerprint).First(&user).Error

	if err == gorm.ErrRecordNotFound {
		// User not found - auto-create (no registration screen needed!)
		user = models.User{
			SSHKeyFingerprint: fingerprint,
			SSHPublicKey:      sshKey,
			Anonymous:         false,
		}

		if err := s.db.Create(&user).Error; err != nil {
			return nil, "", fmt.Errorf("failed to create user: %w", err)
		}
	} else if err != nil {
		return nil, "", fmt.Errorf("database error: %w", err)
	}

	// Generate JWT token for this session
	token, err := s.jwtManager.GenerateToken(user.ID, "", fingerprint[:8])
	if err != nil {
		return nil, "", fmt.Errorf("failed to generate token: %w", err)
	}

	return &user, token, nil
}

// RegisterWithSSHKey creates a new user account with the given SSH key
// No username required - matches terminal.shop pattern
func (s *SSHAuthService) RegisterWithSSHKey(publicKey gossh.PublicKey, name, email string) (*models.User, string, error) {
	fingerprint := GetSSHKeyFingerprint(publicKey)
	sshKeyFormatted := FormatSSHPublicKey(publicKey)

	// Check if SSH key is already registered
	var existing models.User
	err := s.db.Where("ssh_key_fingerprint = ?", fingerprint).First(&existing).Error
	if err == nil {
		return nil, "", fmt.Errorf("SSH key already registered")
	}
	if err != gorm.ErrRecordNotFound {
		return nil, "", fmt.Errorf("database error: %w", err)
	}

	// Create new user (no username required!)
	user := models.User{
		SSHPublicKey:      sshKeyFormatted,
		SSHKeyFingerprint: fingerprint,
		Name:              name,
		Email:             email,
	}

	if err := s.db.Create(&user).Error; err != nil {
		return nil, "", fmt.Errorf("failed to create user: %w", err)
	}

	// Generate JWT token
	token, err := s.jwtManager.GenerateToken(user.ID, "", fingerprint[:8])
	if err != nil {
		return nil, "", fmt.Errorf("failed to generate token: %w", err)
	}

	return &user, token, nil
}

// IsSSHKeyRegistered checks if an SSH key is already registered
func (s *SSHAuthService) IsSSHKeyRegistered(publicKey gossh.PublicKey) (bool, error) {
	fingerprint := GetSSHKeyFingerprint(publicKey)

	var user models.User
	err := s.db.Where("ssh_key_fingerprint = ?", fingerprint).First(&user).Error

	if err == gorm.ErrRecordNotFound {
		return false, nil
	}

	if err != nil {
		return false, fmt.Errorf("database error: %w", err)
	}

	return true, nil
}
