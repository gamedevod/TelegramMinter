package service

import (
	"context"
	"fmt"
	"log"
	"strings"
	"sync"
	"time"

	"stickersbot/internal/client"
	"stickersbot/internal/config"
	"stickersbot/internal/telegram"
)

// TokenInfo token information with caching
type TokenInfo struct {
	Token     string    `json:"token"`
	ExpiresAt time.Time `json:"expires_at"`
	IsValid   bool      `json:"is_valid"`
	LastCheck time.Time `json:"last_check"`
}

// TokenManager manages Bearer tokens for accounts with caching
type TokenManager struct {
	config      *config.Config
	httpClient  *client.HTTPClient
	tokens      map[string]*TokenInfo // key - account name
	mutex       sync.RWMutex
	authService *AuthIntegration

	// Cache settings
	tokenTTL      time.Duration // Token lifetime (default 40 minutes)
	checkCooldown time.Duration // Minimum interval between checks (default 1 minute)
}

// NewTokenManager creates a new token manager
func NewTokenManager(cfg *config.Config) *TokenManager {
	return &TokenManager{
		config:        cfg,
		httpClient:    client.New(),
		tokens:        make(map[string]*TokenInfo),
		authService:   NewAuthIntegration(cfg),
		tokenTTL:      40 * time.Minute, // Tokens live ~45 minutes, refresh 5 minutes before expiration
		checkCooldown: 1 * time.Minute,  // Don't check more often than once per minute
	}
}

// GetCachedToken returns cached token without API check
func (tm *TokenManager) GetCachedToken(accountName string) (string, error) {
	tm.mutex.RLock()
	defer tm.mutex.RUnlock()

	// Find account in configuration
	var account *config.Account
	for _, acc := range tm.config.Accounts {
		if acc.Name == accountName {
			account = &acc
			break
		}
	}

	if account == nil {
		return "", fmt.Errorf("account %s not found", accountName)
	}

	// Check cached token
	if tokenInfo, exists := tm.tokens[accountName]; exists {
		// If token hasn't expired according to our TTL, return it
		if time.Now().Before(tokenInfo.ExpiresAt) {
			return tokenInfo.Token, nil
		}
	}

	// If no cache or token expired, return token from configuration
	if account.AuthToken != "" {
		// Update cache with current token
		tm.tokens[accountName] = &TokenInfo{
			Token:     account.AuthToken,
			ExpiresAt: time.Now().Add(tm.tokenTTL),
			IsValid:   true,
			LastCheck: time.Now(),
		}
		return account.AuthToken, nil
	}

	return "", fmt.Errorf("token for account %s is missing", accountName)
}

// RefreshTokenOnError refreshes token only when receiving authorization error
func (tm *TokenManager) RefreshTokenOnError(accountName string, statusCode int) (string, error) {
	tm.mutex.Lock()
	defer tm.mutex.Unlock()

	log.Printf("🔄 Refreshing token for %s due to error %d", accountName, statusCode)

	// Check cooldown - don't update too often, BUT ignore cooldown for critical token errors
	isTokenError := statusCode == 401 || statusCode == 403 || statusCode == 200 // 200 may contain JSON token error
	if tokenInfo, exists := tm.tokens[accountName]; exists && !isTokenError {
		if time.Since(tokenInfo.LastCheck) < tm.checkCooldown {
			log.Printf("⏳ Token refresh too frequent for %s, using cached", accountName)
			return tokenInfo.Token, nil
		}
	}

	// For token errors, always try to refresh
	if isTokenError {
		log.Printf("🔑 Critical token error for %s (status %d), forced refresh", accountName, statusCode)
	}

	// Find account in configuration
	var account *config.Account
	var accountIndex int
	for i, acc := range tm.config.Accounts {
		if acc.Name == accountName {
			account = &acc
			accountIndex = i
			break
		}
	}

	if account == nil {
		return "", fmt.Errorf("account %s not found", accountName)
	}

	// Refresh token through Telegram authentication
	log.Printf("🔄 Starting Telegram authentication for %s...", accountName)
	newToken, err := tm.refreshTokenViaTelegram(account)
	if err != nil {
		log.Printf("❌ Error refreshing token for %s: %v", accountName, err)
		// Return old token if refresh failed
		if account.AuthToken != "" {
			log.Printf("🔄 Using old token for %s", accountName)
			return account.AuthToken, nil
		}
		return "", fmt.Errorf("error refreshing token for %s: %v", accountName, err)
	}

	tokenPreview := newToken
	if len(tokenPreview) > 20 {
		tokenPreview = tokenPreview[:20] + "..."
	}
	log.Printf("✅ Received new token for %s: %s", accountName, tokenPreview)

	// Check if new token is different from old one
	if account.AuthToken == newToken {
		log.Printf("⚠️ New token for %s is identical to old one! Possible authentication issue", accountName)
	}

	// Check if token is temporary/invalid (only for explicitly temporary tokens)
	if strings.Contains(newToken, "INVALID_TEMP_TOKEN") {
		log.Printf("❌ Received temporary/invalid token for %s: %s", accountName, tokenPreview)
		log.Printf("❌ This token will NOT work with API!")
		return "", fmt.Errorf("received invalid temporary token for %s", accountName)
	}

	// Save new token to configuration
	tm.config.Accounts[accountIndex].AuthToken = newToken

	// Save configuration in background (don't block main thread)
	go func() {
		if err := tm.config.Save("config.json"); err != nil {
			log.Printf("⚠️ Failed to save configuration: %v", err)
		}
	}()

	// Update cache
	tm.tokens[accountName] = &TokenInfo{
		Token:     newToken,
		ExpiresAt: time.Now().Add(tm.tokenTTL),
		IsValid:   true,
		LastCheck: time.Now(),
	}

	log.Printf("✅ Token for account %s successfully updated", accountName)
	return newToken, nil
}

// refreshTokenViaTelegram refreshes token through Telegram authentication
func (tm *TokenManager) refreshTokenViaTelegram(account *config.Account) (string, error) {
	if account.PhoneNumber == "" {
		return "", fmt.Errorf("phone number not specified for account %s", account.Name)
	}

	// Determine session file path
	sessionFile := account.SessionFile
	if sessionFile == "" {
		cleanPhone := strings.ReplaceAll(account.PhoneNumber, "+", "")
		sessionFile = fmt.Sprintf("sessions/%s.session", cleanPhone)
	}

	// Create authentication service
	authService := telegram.NewAuthService(
		tm.config.APIId,
		tm.config.APIHash,
		account.PhoneNumber,
		sessionFile,
	)

	// Execute authentication with timeout
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	bearerToken, err := authService.AuthorizeAndGetToken(ctx)
	if err != nil {
		return "", fmt.Errorf("Telegram authentication error: %v", err)
	}

	return bearerToken, nil
}

// PreventiveRefresh proactively refreshes tokens that are about to expire
func (tm *TokenManager) PreventiveRefresh() {
	tm.mutex.Lock()
	defer tm.mutex.Unlock()

	log.Printf("🔄 Proactively refreshing tokens...")

	for accountName, tokenInfo := range tm.tokens {
		// Refresh tokens that will expire in the next 5 minutes
		if time.Until(tokenInfo.ExpiresAt) < 5*time.Minute {
			log.Printf("⏰ Token for %s is about to expire, refreshing proactively", accountName)

			// Start refresh in separate goroutine to not block
			go func(name string) {
				_, err := tm.RefreshTokenOnError(name, 401) // Forced refresh
				if err != nil {
					log.Printf("❌ Error proactively refreshing token for %s: %v", name, err)
				}
			}(accountName)
		}
	}
}

// GetValidToken returns valid token (main method for use)
func (tm *TokenManager) GetValidToken(accountName string) (string, error) {
	return tm.GetCachedToken(accountName)
}

// InitializeTokens initializes token cache from configuration
func (tm *TokenManager) InitializeTokens() {
	tm.mutex.Lock()
	defer tm.mutex.Unlock()

	log.Printf("🔧 Initializing token cache...")

	for _, account := range tm.config.Accounts {
		if account.AuthToken != "" {
			tm.tokens[account.Name] = &TokenInfo{
				Token:     account.AuthToken,
				ExpiresAt: time.Now().Add(tm.tokenTTL),
				IsValid:   true,
				LastCheck: time.Now(),
			}
			log.Printf("📋 Token for %s added to cache", account.Name)
		}
	}
}

// RefreshTokenOnJSONError refreshes token when receiving JSON token error
func (tm *TokenManager) RefreshTokenOnJSONError(accountName string) (string, error) {
	log.Printf("🔑 Refreshing token for %s due to JSON token error", accountName)
	return tm.RefreshTokenOnError(accountName, 200) // Use status 200 for JSON errors
}

// ForceRefreshToken forcibly refreshes token (ignoring cache and cooldown)
func (tm *TokenManager) ForceRefreshToken(accountName string) (string, error) {
	tm.mutex.Lock()
	defer tm.mutex.Unlock()

	log.Printf("🔄 Forcibly refreshing token for %s", accountName)

	// Find account in configuration
	var account *config.Account
	var accountIndex int
	for i, acc := range tm.config.Accounts {
		if acc.Name == accountName {
			account = &acc
			accountIndex = i
			break
		}
	}

	if account == nil {
		return "", fmt.Errorf("account %s not found", accountName)
	}

	// Refresh token through Telegram authentication
	newToken, err := tm.refreshTokenViaTelegram(account)
	if err != nil {
		log.Printf("❌ Error forcibly refreshing token for %s: %v", accountName, err)
		return "", fmt.Errorf("error refreshing token for %s: %v", accountName, err)
	}

	// Save new token to configuration
	tm.config.Accounts[accountIndex].AuthToken = newToken

	// Save configuration
	if err := tm.config.Save("config.json"); err != nil {
		log.Printf("⚠️ Failed to save configuration: %v", err)
	}

	// Update cache
	tm.tokens[accountName] = &TokenInfo{
		Token:     newToken,
		ExpiresAt: time.Now().Add(tm.tokenTTL),
		IsValid:   true,
		LastCheck: time.Now(),
	}

	log.Printf("✅ Token for account %s forcibly updated", accountName)
	return newToken, nil
}

// InvalidateTokenCache clears token cache for account
func (tm *TokenManager) InvalidateTokenCache(accountName string) {
	tm.mutex.Lock()
	defer tm.mutex.Unlock()

	delete(tm.tokens, accountName)
	log.Printf("🗑️ Token cache for %s cleared", accountName)
}

// ReloadTokenFromConfig reloads token from configuration
func (tm *TokenManager) ReloadTokenFromConfig(accountName string) error {
	tm.mutex.Lock()
	defer tm.mutex.Unlock()

	// Find account in configuration
	var account *config.Account
	for _, acc := range tm.config.Accounts {
		if acc.Name == accountName {
			account = &acc
			break
		}
	}

	if account == nil {
		return fmt.Errorf("account %s not found", accountName)
	}

	if account.AuthToken == "" {
		return fmt.Errorf("token for account %s is missing in configuration", accountName)
	}

	// Update cache with token from configuration
	tm.tokens[accountName] = &TokenInfo{
		Token:     account.AuthToken,
		ExpiresAt: time.Now().Add(tm.tokenTTL),
		IsValid:   true,
		LastCheck: time.Now(),
	}

	log.Printf("🔄 Token for %s reloaded from configuration", accountName)
	return nil
}
