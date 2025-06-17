package service

import (
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"

	"stickersbot/internal/config"
	"stickersbot/internal/storage"
	"stickersbot/internal/telegram"
)

// AuthIntegration integrates Telegram authentication into the main service
type AuthIntegration struct {
	config       *config.Config
	tokenStorage *storage.TokenStorage
}

// NewAuthIntegration creates a new integration service
func NewAuthIntegration(cfg *config.Config, ts *storage.TokenStorage) *AuthIntegration {
	return &AuthIntegration{config: cfg, tokenStorage: ts}
}

// AuthorizeAccounts performs authorization for all accounts that require it
func (ai *AuthIntegration) AuthorizeAccounts(ctx context.Context) error {
	for i, account := range ai.config.Accounts {
		if ai.needsTelegramAuth(account) {
			log.Printf("🔐 Telegram authorization for account: %s", account.Name)

			// Validate account API credentials
			if account.APIId == 0 {
				return fmt.Errorf("account %s: API ID not specified", account.Name)
			}

			if account.APIHash == "" {
				return fmt.Errorf("account %s: API Hash not specified", account.Name)
			}

			// Determine session file path
			sessionFile := account.SessionFile
			if sessionFile == "" {
				// Create session filename based on phone number
				cleanPhone := strings.ReplaceAll(account.PhoneNumber, "+", "")
				sessionFile = filepath.Join("sessions", fmt.Sprintf("%s.session", cleanPhone))
			}

			// Create sessions directory if it doesn't exist
			sessionDir := filepath.Dir(sessionFile)
			if err := os.MkdirAll(sessionDir, 0755); err != nil {
				return fmt.Errorf("creating sessions directory %s: %w", sessionDir, err)
			}

			log.Printf("📁 Session file will be created/used: %s", sessionFile)

			// Create authorization service with account's individual API credentials
			authService := telegram.NewAuthService(
				account.APIId,
				account.APIHash,
				account.PhoneNumber,
				sessionFile,
				account.TwoFactorPassword,
			)

			// Perform authorization
			bearerToken, err := authService.AuthorizeAndGetToken(ctx)
			if err != nil {
				return fmt.Errorf("error authorizing account %s: %w", account.Name, err)
			}

			// Save received token in memory and persist separately
			ai.config.Accounts[i].AuthToken = bearerToken

			if err := ai.tokenStorage.SetToken(account.Name, bearerToken); err != nil {
				log.Printf("⚠️  Failed to store token for %s: %v", account.Name, err)
			}

			log.Printf("✅ Authorization completed for account: %s", account.Name)
		} else if account.AuthToken != "" {
			log.Printf("✅ Account %s already has Bearer token", account.Name)
		} else {
			log.Printf("⚠️  Account %s is not configured for Telegram authorization", account.Name)
		}
	}

	return nil
}

// ValidateAccounts checks the correctness of Telegram authorization settings
func (ai *AuthIntegration) ValidateAccounts() []error {
	var errors []error

	for _, account := range ai.config.Accounts {
		if ai.needsTelegramAuth(account) {
			if account.APIId == 0 {
				errors = append(errors, fmt.Errorf("account %s: API ID not specified", account.Name))
			}

			if account.APIHash == "" {
				errors = append(errors, fmt.Errorf("account %s: API Hash not specified", account.Name))
			}

			if account.PhoneNumber == "" {
				errors = append(errors, fmt.Errorf("account %s: phone_number not specified", account.Name))
			}
		}
	}

	return errors
}

// hasTelegramAuth checks if Telegram authorization is configured for the account
func (ai *AuthIntegration) hasTelegramAuth(account config.Account) bool {
	return account.PhoneNumber != "" &&
		account.APIId != 0 &&
		account.APIHash != ""
}

// needsTelegramAuth checks if Telegram authorization is needed for the account
func (ai *AuthIntegration) needsTelegramAuth(account config.Account) bool {
	return account.AuthToken == "" && ai.hasTelegramAuth(account)
}
