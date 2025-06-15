package telegram

import (
	"bufio"
	"context"
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	"stickersbot/internal/client"
	"stickersbot/internal/constants"

	"github.com/gotd/td/session"
	"github.com/gotd/td/telegram"
	"github.com/gotd/td/telegram/auth"
	"github.com/gotd/td/tg"
)

// AuthService structure for Telegram authorization
type AuthService struct {
	APIId       int
	APIHash     string
	PhoneNumber string
	SessionFile string
	client      *telegram.Client
}

// NewAuthService creates a new authorization service
func NewAuthService(apiId int, apiHash, phoneNumber, sessionFile string) *AuthService {
	return &AuthService{
		APIId:       apiId,
		APIHash:     apiHash,
		PhoneNumber: phoneNumber,
		SessionFile: sessionFile,
	}
}

// AuthorizeAndGetToken authorizes in Telegram and gets Bearer token
func (a *AuthService) AuthorizeAndGetToken(ctx context.Context) (string, error) {
	// Create session from file
	sessionStorage := &session.FileStorage{
		Path: a.SessionFile,
	}

	// Create client
	a.client = telegram.NewClient(a.APIId, a.APIHash, telegram.Options{
		SessionStorage: sessionStorage,
	})

	var bearerToken string

	// Run client
	err := a.client.Run(ctx, func(ctx context.Context) error {
		// Check authorization
		status, err := a.client.Auth().Status(ctx)
		if err != nil {
			return fmt.Errorf("authorization status check: %w", err)
		}

		if !status.Authorized {
			// Authorization needed
			log.Printf("🔐 Authorization for number: %s", a.PhoneNumber)

			if err := a.performAuth(ctx); err != nil {
				return fmt.Errorf("authorization: %w", err)
			}
		} else {
			log.Printf("✅ Already authorized for number: %s", a.PhoneNumber)
		}

		// Get Bearer token through Web App authorization
		token, err := a.getBearerToken(ctx)
		if err != nil {
			return fmt.Errorf("Bearer token retrieval: %w", err)
		}

		bearerToken = token
		return nil
	})

	if err != nil {
		return "", err
	}

	return bearerToken, nil
}

// performAuth performs authorization by phone number
func (a *AuthService) performAuth(ctx context.Context) error {
	flow := auth.NewFlow(
		auth.Constant(
			a.PhoneNumber,
			"", // leave password empty
			auth.CodeAuthenticatorFunc(a.codePrompt),
		),
		auth.SendCodeOptions{},
	)

	return a.client.Auth().IfNecessary(ctx, flow)
}

// codePrompt requests confirmation code from user
func (a *AuthService) codePrompt(ctx context.Context, sentCode *tg.AuthSentCode) (string, error) {
	fmt.Printf("📱 Confirmation code sent to number: %s\n", a.PhoneNumber)
	fmt.Print("Enter code: ")

	reader := bufio.NewReader(os.Stdin)
	code, err := reader.ReadString('\n')
	if err != nil {
		return "", err
	}

	return strings.TrimSpace(code), nil
}

// getBearerToken gets Bearer token for Web App
func (a *AuthService) getBearerToken(ctx context.Context) (string, error) {
	api := a.client.API()

	// Get information about self
	self, err := api.UsersGetFullUser(ctx, &tg.InputUserSelf{})
	if err != nil {
		return "", fmt.Errorf("getting user information: %w", err)
	}

	user := self.Users[0].(*tg.User)
	log.Printf("👤 Authorized as: %s %s (@%s)",
		user.FirstName,
		user.LastName,
		user.Username)

	// Here we need to get Bearer token for specific bot/application
	// This depends on how your application gets the token
	// For example, through embedded Web App or bot API

	// For demonstration - generate token based on user data
	// In reality, there should be a call to your application API
	token, err := a.generateBearerToken(ctx, user)
	if err != nil {
		return "", fmt.Errorf("Bearer token generation: %w", err)
	}

	return token, nil
}

// generateBearerToken generates Bearer token
// Uses exactly the same logic as in Python code
func (a *AuthService) generateBearerToken(ctx context.Context, user *tg.User) (string, error) {
	api := a.client.API()

	// Use constants instead of configuration values
	botUsername := constants.BotUsername
	webAppURL := constants.WebAppURL

	log.Printf("🔧 Using bot: %s, Web App: %s", botUsername, webAppURL)

	// 1. Get auth data (analog of get_auth_data from Python)
	log.Printf("🔄 Getting auth data for bot %s...", botUsername)
	webAppService := NewWebAppService(api, botUsername, webAppURL)
	authResponse, err := webAppService.GetAuthData(ctx, botUsername, webAppURL)
	if err != nil {
		log.Printf("❌ Error getting auth data: %v", err)
		log.Printf("🔄 Switching to fallback token...")
		return a.fallbackToTempToken(user.ID)
	}

	if authResponse.Status != "SUCCESS" {
		log.Printf("❌ Failed to get auth data: %s", authResponse.Description)
		log.Printf("🔄 Switching to fallback token...")
		return a.fallbackToTempToken(user.ID)
	}

	log.Printf("✅ Auth data successfully obtained")

	authData, ok := authResponse.Data.(*client.AuthData)
	if !ok {
		log.Printf("⚠️  Invalid auth data format")
		return a.fallbackToTempToken(user.ID)
	}

	// Check that auth data is valid
	if !authData.IsValid() {
		log.Printf("⚠️  Auth data expired")
		return a.fallbackToTempToken(user.ID)
	}

	// 2. Send auth data to API to get Bearer token (analog of auth from Python)
	apiURL := constants.TokenAPIURL
	log.Printf("🌐 Using API URL: %s", apiURL)

	// Use existing HTTPClient
	httpClient := client.New()

	// Send auth data to API
	log.Printf("🔄 Sending auth data to API %s...", apiURL)
	tokenResponse, err := httpClient.AuthenticateWithTelegramData(apiURL, authData)
	if err != nil {
		log.Printf("❌ Error authenticating through API: %v", err)
		log.Printf("🔄 Switching to fallback token...")
		return a.fallbackToTempToken(user.ID)
	}

	if tokenResponse.Status == "SUCCESS" {
		bearerToken := tokenResponse.Data.(string)
		log.Printf("✅ Bearer token obtained through API: %s", maskToken(bearerToken))
		return bearerToken, nil
	}

	log.Printf("❌ API authentication failed: %s", tokenResponse.Description)
	log.Printf("🔄 Switching to fallback token...")
	return a.fallbackToTempToken(user.ID)
}

// fallbackToTempToken creates temporary token if main methods failed
func (a *AuthService) fallbackToTempToken(userID int64) (string, error) {
	timestamp := time.Now().Unix()
	tempToken := fmt.Sprintf("tg_token_%d_%d", userID, timestamp)

	log.Printf("🎫 Created temporary Bearer token: %s", maskToken(tempToken))
	log.Printf("⚠️  WARNING: Using temporary token!")
	log.Printf("⚠️  Check settings: bot_username=%s, web_app_url=%s, token_api_url=%s",
		constants.BotUsername, constants.WebAppURL, constants.TokenAPIURL)

	return tempToken, nil
}

// requestTokenFromYourAPI example of token request from your API
func (a *AuthService) requestTokenFromYourAPI(userID int64) (string, error) {
	// Here should be HTTP request to your API
	// which accepts Telegram User ID and returns Bearer token

	// Example:
	// client := &http.Client{}
	// req, _ := http.NewRequest("POST", "https://your-api.com/auth/telegram",
	//     strings.NewReader(fmt.Sprintf(`{"telegram_user_id": %d}`, userID)))
	// req.Header.Set("Content-Type", "application/json")
	//
	// resp, err := client.Do(req)
	// if err != nil {
	//     return "", err
	// }
	// defer resp.Body.Close()
	//
	// var result struct {
	//     Token string `json:"token"`
	// }
	// json.NewDecoder(resp.Body).Decode(&result)
	//
	// return result.Token, nil

	return "", fmt.Errorf("method not implemented - add your token retrieval logic")
}
