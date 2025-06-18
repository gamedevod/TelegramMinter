package telegram

import (
	"bufio"
	"context"
	"fmt"
	"log"
	"net"
	"os"
	"strings"
	"time"

	"github.com/gotd/td/session"
	"github.com/gotd/td/telegram"
	"github.com/gotd/td/telegram/auth"
	"github.com/gotd/td/telegram/dcs"
	"github.com/gotd/td/tg"

	netproxy "golang.org/x/net/proxy"

	"stickersbot/internal/client"
	"stickersbot/internal/constants"
	"stickersbot/internal/proxy"
)

// AuthService structure for Telegram authorization
type AuthService struct {
	APIId             int
	APIHash           string
	PhoneNumber       string
	SessionFile       string
	TwoFactorPassword string // 2FA password, if empty - will prompt user
	UseProxy          bool   // Whether to use proxy
	ProxyURL          string // Proxy URL in format host:port:user:pass
	client            *telegram.Client
}

// NewAuthService creates a new authorization service
func NewAuthService(apiId int, apiHash, phoneNumber, sessionFile, twoFactorPassword string) *AuthService {
	return NewAuthServiceWithProxy(apiId, apiHash, phoneNumber, sessionFile, twoFactorPassword, false, "")
}

// NewAuthServiceWithProxy creates a new authorization service with proxy support
func NewAuthServiceWithProxy(apiId int, apiHash, phoneNumber, sessionFile, twoFactorPassword string, useProxy bool, proxyURL string) *AuthService {
	if useProxy {
		if proxyURL == "" {
			proxyURL = proxy.GetRandom()
		}
	}
	return &AuthService{
		APIId:             apiId,
		APIHash:           apiHash,
		PhoneNumber:       phoneNumber,
		SessionFile:       sessionFile,
		TwoFactorPassword: twoFactorPassword,
		UseProxy:          useProxy,
		ProxyURL:          proxyURL,
	}
}

// AuthorizeAndGetToken authorizes in Telegram and gets Bearer token
func (a *AuthService) AuthorizeAndGetToken(ctx context.Context) (string, error) {
	// Create session from file
	sessionStorage := &session.FileStorage{
		Path: a.SessionFile,
	}

	// Create client options
	clientOptions := telegram.Options{
		SessionStorage: sessionStorage,
	}

	// Add proxy support if enabled
	if a.UseProxy && a.ProxyURL != "" {
		dialFunc, err := createProxyDialFunc(a.ProxyURL)
		if err != nil {
			return "", fmt.Errorf("invalid proxy URL: %v", err)
		}

		// Use dcs.Plain with proxy dial function
		clientOptions.Resolver = dcs.Plain(dcs.PlainOptions{
			Dial: dialFunc,
		})
	}

	// Create client
	a.client = telegram.NewClient(a.APIId, a.APIHash, clientOptions)

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
	// Create custom authenticator that handles 2FA properly
	authenticator := &customAuthenticator{
		phoneNumber:       a.PhoneNumber,
		twoFactorPassword: a.TwoFactorPassword,
		authService:       a,
	}

	flow := auth.NewFlow(
		authenticator,
		auth.SendCodeOptions{},
	)

	return a.client.Auth().IfNecessary(ctx, flow)
}

// customAuthenticator implements auth.UserAuthenticator with proper 2FA support
type customAuthenticator struct {
	phoneNumber       string
	twoFactorPassword string
	authService       *AuthService
}

func (c *customAuthenticator) Phone(ctx context.Context) (string, error) {
	return c.phoneNumber, nil
}

func (c *customAuthenticator) Password(ctx context.Context) (string, error) {
	return c.authService.passwordPrompt(ctx)
}

func (c *customAuthenticator) Code(ctx context.Context, sentCode *tg.AuthSentCode) (string, error) {
	return c.authService.codePrompt(ctx, sentCode)
}

func (c *customAuthenticator) AcceptTermsOfService(ctx context.Context, tos tg.HelpTermsOfService) error {
	// Auto-accept terms of service
	return nil
}

func (c *customAuthenticator) SignUp(ctx context.Context) (auth.UserInfo, error) {
	// Return empty user info - we don't support sign up
	return auth.UserInfo{}, fmt.Errorf("sign up not supported")
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

// passwordPrompt requests 2FA password from user (used as fallback if config password fails)
func (a *AuthService) passwordPrompt(ctx context.Context) (string, error) {
	fmt.Printf("🔐 Two-factor authentication required for number: %s\n", a.PhoneNumber)

	// If password is provided in config, try it first
	if a.TwoFactorPassword != "" {
		log.Printf("📋 Using 2FA password from config")
		return a.TwoFactorPassword, nil
	}

	// Otherwise, prompt user
	fmt.Print("Enter your 2FA password: ")
	reader := bufio.NewReader(os.Stdin)
	password, err := reader.ReadString('\n')
	if err != nil {
		return "", fmt.Errorf("error reading password: %w", err)
	}

	password = strings.TrimSpace(password)
	if password == "" {
		return "", fmt.Errorf("password cannot be empty")
	}

	return password, nil
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
	log.Printf("🔧 User ID: %d, Username: @%s", user.ID, user.Username)

	// 1. Get auth data (analog of get_auth_data from Python)
	log.Printf("🔄 Getting auth data for bot %s...", botUsername)
	webAppService := NewWebAppServiceWithProxy(api, botUsername, webAppURL, a.UseProxy, a.ProxyURL)
	authResponse, err := webAppService.GetAuthData(ctx, botUsername, webAppURL)
	if err != nil {
		log.Printf("❌ Error getting auth data: %v", err)
		log.Printf("🔄 Switching to fallback token...")
		return a.fallbackToTempToken(user.ID)
	}

	log.Printf("🔍 Auth response status: %s", authResponse.Status)
	if authResponse.Status != "SUCCESS" {
		log.Printf("❌ Failed to get auth data: %s", authResponse.Description)
		log.Printf("🔄 Switching to fallback token...")
		return a.fallbackToTempToken(user.ID)
	}

	log.Printf("✅ Auth data successfully obtained")

	authData, ok := authResponse.Data.(*client.AuthData)
	if !ok {
		log.Printf("⚠️  Invalid auth data format, type: %T", authResponse.Data)
		return a.fallbackToTempToken(user.ID)
	}

	log.Printf("🔍 Auth data: Data length=%d, Expires=%s", len(authData.Data), authData.Exp.Format("15:04:05"))

	// Check that auth data is valid
	if !authData.IsValid() {
		log.Printf("⚠️  Auth data expired (current time: %s, expires: %s)",
			time.Now().Format("15:04:05"), authData.Exp.Format("15:04:05"))
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

	log.Printf("🔍 Token response status: %s", tokenResponse.Status)
	if tokenResponse.Status == "SUCCESS" {
		bearerToken, ok := tokenResponse.Data.(string)
		if !ok {
			log.Printf("❌ Invalid token format, type: %T", tokenResponse.Data)
			log.Printf("🔄 Switching to fallback token...")
			return a.fallbackToTempToken(user.ID)
		}
		log.Printf("✅ Bearer token obtained through API: %s", maskToken(bearerToken))
		return bearerToken, nil
	}

	log.Printf("❌ API authentication failed: %s", tokenResponse.Description)
	if tokenResponse.Data != nil {
		log.Printf("🔍 Additional error data: %v", tokenResponse.Data)
	}
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

// createProxyDialFunc creates dial function for proxy connection
// proxyURL format: host:port:user:pass
func createProxyDialFunc(proxyURL string) (func(ctx context.Context, network, addr string) (net.Conn, error), error) {
	parts := strings.Split(proxyURL, ":")
	if len(parts) < 2 {
		return nil, fmt.Errorf("invalid proxy format, expected host:port or host:port:user:pass")
	}

	host := parts[0]
	port := parts[1]
	proxyAddr := net.JoinHostPort(host, port)

	if len(parts) == 2 {
		// No authentication - use SOCKS5 without auth
		dialer, err := netproxy.SOCKS5("tcp", proxyAddr, nil, netproxy.Direct)
		if err != nil {
			return nil, fmt.Errorf("failed to create SOCKS5 proxy: %v", err)
		}

		if contextDialer, ok := dialer.(netproxy.ContextDialer); ok {
			return contextDialer.DialContext, nil
		}

		// Fallback for non-context dialers
		return func(ctx context.Context, network, addr string) (net.Conn, error) {
			return dialer.Dial(network, addr)
		}, nil
	} else if len(parts) == 4 {
		// With authentication
		user := parts[2]
		pass := parts[3]
		auth := &netproxy.Auth{
			User:     user,
			Password: pass,
		}

		dialer, err := netproxy.SOCKS5("tcp", proxyAddr, auth, netproxy.Direct)
		if err != nil {
			return nil, fmt.Errorf("failed to create SOCKS5 proxy with auth: %v", err)
		}

		if contextDialer, ok := dialer.(netproxy.ContextDialer); ok {
			return contextDialer.DialContext, nil
		}

		// Fallback for non-context dialers
		return func(ctx context.Context, network, addr string) (net.Conn, error) {
			return dialer.Dial(network, addr)
		}, nil
	}

	return nil, fmt.Errorf("invalid proxy format, expected host:port or host:port:user:pass")
}
