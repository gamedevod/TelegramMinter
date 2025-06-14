package service

import (
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"

	"stickersbot/internal/config"
	"stickersbot/internal/telegram"
)

// AuthIntegration integrates Telegram authentication into the main service
type AuthIntegration struct {
	config *config.Config
}

// NewAuthIntegration creates a new integration service
func NewAuthIntegration(cfg *config.Config) *AuthIntegration {
	return &AuthIntegration{config: cfg}
}

// AuthorizeAccounts performs authorization for all accounts that require it
func (ai *AuthIntegration) AuthorizeAccounts(ctx context.Context) error {
	for i, account := range ai.config.Accounts {
		if ai.needsTelegramAuth(account) {
			log.Printf("🔐 Telegram authorization for account: %s", account.Name)

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

			// Create authorization service with common parameters
			authService := telegram.NewAuthService(
				ai.config.APIId,
				ai.config.APIHash,
				account.PhoneNumber,
				sessionFile,
				ai.config.BotUsername,
				ai.config.WebAppURL,
				ai.config.TokenAPIURL,
			)

			// Perform authorization
			bearerToken, err := authService.AuthorizeAndGetToken(ctx)
			if err != nil {
				return fmt.Errorf("error authorizing account %s: %w", account.Name, err)
			}

			// Save received token
			ai.config.Accounts[i].AuthToken = bearerToken
			log.Printf("✅ Authorization completed for account: %s", account.Name)
		} else if account.AuthToken != "" {
			log.Printf("✅ Account %s already has Bearer token", account.Name)
		} else {
			log.Printf("⚠️  Account %s is not configured for Telegram authorization", account.Name)
		}
	}

	// Save configuration with received tokens
	if err := ai.saveConfig(); err != nil {
		log.Printf("⚠️  Failed to save configuration: %v", err)
	}

	return nil
}

// ValidateAccounts checks the correctness of Telegram authorization settings
func (ai *AuthIntegration) ValidateAccounts() []error {
	var errors []error

	for _, account := range ai.config.Accounts {
		if ai.needsTelegramAuth(account) {
			if ai.config.APIId == 0 {
				errors = append(errors, fmt.Errorf("account %s: api_id not specified in common settings", account.Name))
			}

			if ai.config.APIHash == "" {
				errors = append(errors, fmt.Errorf("account %s: api_hash not specified in common settings", account.Name))
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
		ai.config.APIId != 0 &&
		ai.config.APIHash != ""
}

// needsTelegramAuth checks if Telegram authorization is needed for the account
func (ai *AuthIntegration) needsTelegramAuth(account config.Account) bool {
	return account.AuthToken == "" && ai.hasTelegramAuth(account)
}

// saveConfig saves configuration to file
func (ai *AuthIntegration) saveConfig() error {
	return ai.config.Save("config.json")
}
