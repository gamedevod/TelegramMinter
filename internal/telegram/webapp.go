package telegram

import (
	"context"
	"fmt"
	"log"
	"net/url"
	"regexp"
	"strings"
	"time"

	"stickersbot/internal/client"

	"github.com/gotd/td/tg"
)

// WebAppService service for working with Telegram Web App
type WebAppService struct {
	api         *tg.Client
	botUsername string             // bot name for token retrieval
	webAppURL   string             // web application URL
	httpClient  *client.HTTPClient // HTTP client for requests
}

// NewWebAppService creates a new Web App service
func NewWebAppService(api *tg.Client, botUsername, webAppURL string) *WebAppService {
	return &WebAppService{
		api:         api,
		botUsername: botUsername,
		webAppURL:   webAppURL,
		httpClient:  client.New(), // use existing HTTP client
	}
}

// GetBearerTokenFromWebApp gets Bearer token through Web App
func (w *WebAppService) GetBearerTokenFromWebApp(ctx context.Context, userID int64) (string, error) {
	log.Printf("🌐 Requesting Bearer token through Web App for bot: %s", w.botUsername)

	// 1. Find bot
	bot, err := w.findBot(ctx)
	if err != nil {
		return "", fmt.Errorf("bot search: %w", err)
	}

	// 2. Request Web App
	webAppData, err := w.requestWebApp(ctx, bot, userID)
	if err != nil {
		return "", fmt.Errorf("Web App request: %w", err)
	}

	// 3. Extract Bearer token from Web App data
	token, err := w.extractBearerToken(webAppData)
	if err != nil {
		return "", fmt.Errorf("Bearer token extraction: %w", err)
	}

	log.Printf("✅ Bearer token obtained through Web App: %s", maskToken(token))
	return token, nil
}

// GetBearerTokenFromBot gets Bearer token by sending command to bot
func (w *WebAppService) GetBearerTokenFromBot(ctx context.Context, userID int64) (string, error) {
	log.Printf("🤖 Requesting Bearer token through bot command: %s", w.botUsername)

	// 1. Find bot
	bot, err := w.findBot(ctx)
	if err != nil {
		return "", fmt.Errorf("bot search: %w", err)
	}

	// 2. Send /start or /token command to bot
	token, err := w.sendTokenCommand(ctx, bot, userID)
	if err != nil {
		return "", fmt.Errorf("sending command to bot: %w", err)
	}

	log.Printf("✅ Bearer token obtained from bot: %s", maskToken(token))
	return token, nil
}

// findBot finds bot by username
func (w *WebAppService) findBot(ctx context.Context) (*tg.User, error) {
	// Resolve bot username
	resolved, err := w.api.ContactsResolveUsername(ctx, &tg.ContactsResolveUsernameRequest{
		Username: w.botUsername,
	})
	if err != nil {
		return nil, fmt.Errorf("username resolution %s: %w", w.botUsername, err)
	}

	// Search for bot user
	for _, user := range resolved.Users {
		if u, ok := user.(*tg.User); ok && u.Bot {
			return u, nil
		}
	}

	return nil, fmt.Errorf("bot %s not found", w.botUsername)
}

// requestWebApp requests Web App from bot
func (w *WebAppService) requestWebApp(ctx context.Context, bot *tg.User, userID int64) (string, error) {
	// Create input peer for bot
	inputPeer := &tg.InputPeerUser{
		UserID:     bot.ID,
		AccessHash: bot.AccessHash,
	}

	// Create input user for bot
	inputUser := &tg.InputUser{
		UserID:     bot.ID,
		AccessHash: bot.AccessHash,
	}

	// Request Web App
	webView, err := w.api.MessagesRequestWebView(ctx, &tg.MessagesRequestWebViewRequest{
		Peer:     inputPeer,
		Bot:      inputUser,
		URL:      w.webAppURL,
		Platform: "web",
	})
	if err != nil {
		return "", fmt.Errorf("Web App request: %w", err)
	}

	log.Printf("🔗 Web App URL: %s", webView.URL)

	// Return full URL data for further processing
	return webView.URL, nil
}

// sendTokenCommand sends command to bot to get token
func (w *WebAppService) sendTokenCommand(ctx context.Context, bot *tg.User, userID int64) (string, error) {
	// Create input peer for bot
	inputPeer := &tg.InputPeerUser{
		UserID:     bot.ID,
		AccessHash: bot.AccessHash,
	}

	// Send /token or /start command
	_, err := w.api.MessagesSendMessage(ctx, &tg.MessagesSendMessageRequest{
		Peer:    inputPeer,
		Message: "/token",
	})
	if err != nil {
		return "", fmt.Errorf("sending command: %w", err)
	}

	log.Printf("📤 /token command sent to bot")

	// Wait for bot response (simplified version)
	// In reality, need to set up message handler
	time.Sleep(2 * time.Second)

	// Get recent messages
	messages, err := w.api.MessagesGetHistory(ctx, &tg.MessagesGetHistoryRequest{
		Peer:  inputPeer,
		Limit: 10,
	})
	if err != nil {
		return "", fmt.Errorf("getting history: %w", err)
	}

	// Search for token in messages
	if history, ok := messages.(*tg.MessagesMessages); ok {
		for _, msg := range history.Messages {
			if m, ok := msg.(*tg.Message); ok {
				token := extractTokenFromMessage(m.Message)
				if token != "" {
					return token, nil
				}
			}
		}
	}

	return "", fmt.Errorf("token not found in bot responses")
}

// extractBearerToken extracts Bearer token from Web App data
func (w *WebAppService) extractBearerToken(webAppURL string) (string, error) {
	log.Printf("🔍 Analyzing Web App URL: %s", webAppURL)

	// Parse URL and extract data
	parsedURL, err := url.Parse(webAppURL)
	if err != nil {
		return "", fmt.Errorf("URL parsing: %w", err)
	}

	// Check regular query parameters
	queryParams := parsedURL.Query()

	// 1. Check direct tokens in parameters
	tokenParams := []string{"token", "auth_token", "bearer", "access_token", "jwt"}
	for _, param := range tokenParams {
		if token := queryParams.Get(param); token != "" {
			log.Printf("✅ Found token in parameter %s", param)
			return token, nil
		}
	}

	// 2. Check token in hash part of URL (after #)
	if fragment := parsedURL.Fragment; fragment != "" {
		if token := extractTokenFromFragment(fragment); token != "" {
			log.Printf("✅ Found token in fragment")
			return token, nil
		}
	}

	// 3. Extract tgWebAppData/initData from URL
	initData := queryParams.Get("tgWebAppData")
	if initData == "" {
		initData = queryParams.Get("initData")
	}

	if initData != "" {
		log.Printf("🔍 Found initData, sending to API for token")
		return w.requestTokenWithInitData(initData)
	}

	// 4. If this is direct Web App link, try to load it
	if strings.Contains(webAppURL, "tgWebAppData=") || strings.Contains(webAppURL, "initData=") {
		return w.extractInitDataFromURL(webAppURL)
	}

	// 5. Last attempt - request to application API
	return w.requestTokenFromWebAppAPI(webAppURL)
}

// extractInitDataFromURL extracts initData from URL
func (w *WebAppService) extractInitDataFromURL(webAppURL string) (string, error) {
	// Search for tgWebAppData or initData in URL
	re := regexp.MustCompile(`(?:tgWebAppData|initData)=([^&\s#]+)`)
	matches := re.FindStringSubmatch(webAppURL)

	if len(matches) < 2 {
		return "", fmt.Errorf("initData not found in URL")
	}

	initData, err := url.QueryUnescape(matches[1])
	if err != nil {
		return "", fmt.Errorf("initData decoding error: %w", err)
	}

	log.Printf("🔍 Extracted initData: %s...", initData[:min(50, len(initData))])

	return w.requestTokenWithInitData(initData)
}

// requestTokenWithInitData sends initData to API to get token
func (w *WebAppService) requestTokenWithInitData(initData string) (string, error) {
	// Here should be HTTP request to your API
	// which accepts initData and returns Bearer token

	log.Printf("📤 Sending initData to application API")

	/* Example implementation:

	import "encoding/json"
	import "net/http"
	import "bytes"

	type InitDataRequest struct {
		InitData string `json:"init_data"`
	}

	type TokenResponse struct {
		Token string `json:"token"`
		Error string `json:"error,omitempty"`
	}

	reqBody := InitDataRequest{InitData: initData}
	jsonData, _ := json.Marshal(reqBody)

	resp, err := http.Post("https://your-api.com/auth/telegram-webapp",
		"application/json", bytes.NewBuffer(jsonData))
	if err != nil {
		return "", fmt.Errorf("API request: %w", err)
	}
	defer resp.Body.Close()

	var tokenResp TokenResponse
	if err := json.NewDecoder(resp.Body).Decode(&tokenResp); err != nil {
		return "", fmt.Errorf("response decoding: %w", err)
	}

	if tokenResp.Error != "" {
		return "", fmt.Errorf("API error: %s", tokenResp.Error)
	}

	return tokenResp.Token, nil
	*/

	// For demonstration - create token based on initData
	// In reality, there should be a call to your API!
	token := fmt.Sprintf("demo_token_%x", initData[:min(8, len(initData))])
	log.Printf("⚠️  DEMO: Created test token: %s", maskToken(token))
	log.Printf("⚠️  WARNING: Implement requestTokenWithInitData for your API!")

	return token, nil
}

// requestTokenFromWebAppAPI makes additional request to application API
func (w *WebAppService) requestTokenFromWebAppAPI(webAppURL string) (string, error) {
	// This function is called if initData is not found
	// You can implement alternative token retrieval logic

	log.Printf("⚠️  Web App URL doesn't contain initData or direct token: %s", webAppURL)
	log.Printf("⚠️  Try:")
	log.Printf("    1. Check bot_username correctness")
	log.Printf("    2. Make sure bot has Web App")
	log.Printf("    3. Check web_app_url in configuration")

	return "", fmt.Errorf("failed to extract token from Web App URL")
}

// min returns minimum value
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// extractTokenFromFragment extracts token from URL fragment part
func extractTokenFromFragment(fragment string) string {
	// Parse fragment as query string
	values, err := url.ParseQuery(fragment)
	if err != nil {
		return ""
	}

	tokenParams := []string{"token", "auth_token", "bearer", "access_token", "jwt"}
	for _, param := range tokenParams {
		if token := values.Get(param); token != "" {
			return token
		}
	}

	return ""
}

// extractTokenFromMessage extracts token from message text
func extractTokenFromMessage(message string) string {
	// Regular expressions for token search
	tokenPatterns := []string{
		`(?i)token[:\s]+([A-Za-z0-9_\-\.]+)`,
		`(?i)bearer[:\s]+([A-Za-z0-9_\-\.]+)`,
		`(?i)auth[:\s]+([A-Za-z0-9_\-\.]+)`,
		`([A-Za-z0-9_\-\.]{32,})`, // Long strings (possible tokens)
	}

	for _, pattern := range tokenPatterns {
		re := regexp.MustCompile(pattern)
		matches := re.FindStringSubmatch(message)
		if len(matches) > 1 {
			return matches[1]
		}
	}

	return ""
}

// maskToken masks token for safe logging
func maskToken(token string) string {
	if len(token) <= 8 {
		return strings.Repeat("*", len(token))
	}
	return token[:4] + strings.Repeat("*", len(token)-8) + token[len(token)-4:]
}

// GetAuthData gets auth data from Telegram Web App (analog of Python function)
func (w *WebAppService) GetAuthData(ctx context.Context, botTag, webAppURL string) (*client.TelegramAuthResponse, error) {
	log.Printf("🔍 Getting auth data for bot: %s", botTag)

	// 1. Find bot
	bot, err := w.findBotByTag(ctx, botTag)
	if err != nil {
		log.Printf("❌ Bot search error: %v", err)
		return &client.TelegramAuthResponse{
			Status:      "ERROR",
			Description: "Bot not found",
			Data:        err,
		}, err
	}

	// 2. Request Web App
	webAppData, err := w.requestWebAppData(ctx, bot, webAppURL)
	if err != nil {
		log.Printf("❌ Error getting Web App data: %v", err)
		return &client.TelegramAuthResponse{
			Status:      "ERROR",
			Description: "Failed to get Web App data",
			Data:        err,
		}, err
	}

	log.Printf("✅ Auth data obtained successfully")
	return &client.TelegramAuthResponse{
		Status:      "SUCCESS",
		Description: "OK",
		Data:        webAppData,
	}, nil
}

// findBotByTag finds bot by tag (analog of resolve_peer)
func (w *WebAppService) findBotByTag(ctx context.Context, botTag string) (*tg.User, error) {
	// Remove @ if present
	botUsername := strings.TrimPrefix(botTag, "@")

	// Resolve bot username
	resolved, err := w.api.ContactsResolveUsername(ctx, &tg.ContactsResolveUsernameRequest{
		Username: botUsername,
	})
	if err != nil {
		return nil, fmt.Errorf("username resolution %s: %w", botUsername, err)
	}

	// Search for bot user
	for _, user := range resolved.Users {
		if u, ok := user.(*tg.User); ok && u.Bot {
			return u, nil
		}
	}

	return nil, fmt.Errorf("bot %s not found", botTag)
}

// requestWebAppData requests Web App data (analog of RequestWebView)
func (w *WebAppService) requestWebAppData(ctx context.Context, bot *tg.User, webAppURL string) (*client.AuthData, error) {
	// Create input peer for bot
	inputPeer := &tg.InputPeerUser{
		UserID:     bot.ID,
		AccessHash: bot.AccessHash,
	}

	// Create input user for bot
	inputUser := &tg.InputUser{
		UserID:     bot.ID,
		AccessHash: bot.AccessHash,
	}

	// Request Web App (analog of RequestWebView from Python)
	webView, err := w.api.MessagesRequestWebView(ctx, &tg.MessagesRequestWebViewRequest{
		Peer:        inputPeer,
		Bot:         inputUser,
		URL:         webAppURL,
		Platform:    "android", // as in Python code
		FromBotMenu: false,     // as in Python code
	})
	if err != nil {
		return nil, fmt.Errorf("Web App request: %w", err)
	}

	log.Printf("🔗 Received Web App URL: %s", webView.URL)

	// Extract tgWebAppData from URL (as in Python)
	authDataString, err := w.extractTgWebAppData(webView.URL)
	if err != nil {
		return nil, fmt.Errorf("tgWebAppData extraction: %w", err)
	}

	// Create AuthData with 45 minutes expiration (as in Python)
	expTime := time.Now().Add(45 * time.Minute)
	authData := client.NewAuthData(authDataString, expTime)

	log.Printf("📋 Auth data extracted, expires: %s", expTime.Format("15:04:05"))

	return authData, nil
}

// extractTgWebAppData extracts and decodes tgWebAppData (analog of Python unquote)
func (w *WebAppService) extractTgWebAppData(webAppURL string) (string, error) {
	// Search for tgWebAppData in URL
	if !strings.Contains(webAppURL, "tgWebAppData=") {
		return "", fmt.Errorf("tgWebAppData not found in URL")
	}

	// Split URL and extract part with tgWebAppData
	parts := strings.Split(webAppURL, "tgWebAppData=")
	if len(parts) < 2 {
		return "", fmt.Errorf("incorrect URL format")
	}

	// Extract data until next parameter
	tgWebAppData := strings.Split(parts[1], "&tgWebAppVersion")[0]

	// Decode URL (analog of Python unquote)
	decoded1, err := url.QueryUnescape(tgWebAppData)
	if err != nil {
		return "", fmt.Errorf("first decoding: %w", err)
	}

	// Second decoding (as in Python - double unquote)
	decoded2, err := url.QueryUnescape(decoded1)
	if err != nil {
		return "", fmt.Errorf("second decoding: %w", err)
	}

	log.Printf("🔓 Decoded auth data: %s...", decoded2[:min(50, len(decoded2))])

	return decoded2, nil
}
