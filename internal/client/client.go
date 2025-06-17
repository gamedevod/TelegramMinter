package client

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"strings"

	fhttp "github.com/bogdanfinn/fhttp"
	tls_client "github.com/bogdanfinn/tls-client"
	"github.com/bogdanfinn/tls-client/profiles"
)

// APIResponse structure for successful API response
type APIResponse struct {
	OK   bool `json:"ok"`
	Data struct {
		OrderID     string `json:"order_id"`
		TotalAmount int64  `json:"total_amount"`
		Currency    string `json:"currency"`
		Wallet      string `json:"wallet"`
	} `json:"data"`
}

// APIErrorResponse structure for error API response
type APIErrorResponse struct {
	OK        bool   `json:"ok"`
	ErrorCode string `json:"errorCode"`
}

// HTTPClient wrapper for tls-client
type HTTPClient struct {
	client tls_client.HttpClient
}

// New creates a new HTTP client without proxy
func New() *HTTPClient {
	jar := tls_client.NewCookieJar()
	options := []tls_client.HttpClientOption{
		tls_client.WithTimeoutSeconds(30),
		tls_client.WithClientProfile(profiles.Chrome_120),
		tls_client.WithRandomTLSExtensionOrder(),
		tls_client.WithNotFollowRedirects(),
		tls_client.WithCookieJar(jar),
	}

	client, err := tls_client.NewHttpClient(tls_client.NewNoopLogger(), options...)
	if err != nil {
		panic(fmt.Sprintf("Error creating HTTP client: %v", err))
	}

	return &HTTPClient{
		client: client,
	}
}

// NewWithProxy creates a new HTTP client with proxy support
// proxyURL format: host:port:user:pass
func NewWithProxy(proxyURL string) (*HTTPClient, error) {
	jar := tls_client.NewCookieJar()

	options := []tls_client.HttpClientOption{
		tls_client.WithTimeoutSeconds(30),
		tls_client.WithClientProfile(profiles.Chrome_120),
		tls_client.WithRandomTLSExtensionOrder(),
		tls_client.WithNotFollowRedirects(),
		tls_client.WithCookieJar(jar),
	}

	// Parse proxy URL if provided
	if proxyURL != "" {
		proxyURLParsed, err := parseProxyURL(proxyURL)
		if err != nil {
			return nil, fmt.Errorf("invalid proxy URL: %v", err)
		}
		options = append(options, tls_client.WithProxyUrl(proxyURLParsed))
	}

	client, err := tls_client.NewHttpClient(tls_client.NewNoopLogger(), options...)
	if err != nil {
		return nil, fmt.Errorf("error creating HTTP client with proxy: %v", err)
	}

	return &HTTPClient{
		client: client,
	}, nil
}

// parseProxyURL parses proxy URL from format host:port:user:pass to standard URL
func parseProxyURL(proxyURL string) (string, error) {
	parts := strings.Split(proxyURL, ":")
	if len(parts) < 2 {
		return "", fmt.Errorf("invalid proxy format, expected host:port or host:port:user:pass")
	}

	host := parts[0]
	port := parts[1]

	if len(parts) == 2 {
		// No authentication
		return fmt.Sprintf("http://%s:%s", host, port), nil
	} else if len(parts) == 4 {
		// With authentication
		user := parts[2]
		pass := parts[3]
		return fmt.Sprintf("http://%s:%s@%s:%s", user, pass, host, port), nil
	}

	return "", fmt.Errorf("invalid proxy format, expected host:port or host:port:user:pass")
}

// Get performs a GET request
func (c *HTTPClient) Get(url string, headers map[string]string) (*fhttp.Response, error) {
	req, err := fhttp.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}

	// Add headers
	for key, value := range headers {
		req.Header.Set(key, value)
	}

	return c.client.Do(req)
}

// Post performs a POST request
func (c *HTTPClient) Post(url string, body string, headers map[string]string) (*fhttp.Response, error) {
	req, err := fhttp.NewRequest("POST", url, strings.NewReader(body))
	if err != nil {
		return nil, err
	}

	// Add headers
	for key, value := range headers {
		req.Header.Set(key, value)
	}

	return c.client.Do(req)
}

// BuyStickersResponse response structure for sticker purchase
type BuyStickersResponse struct {
	StatusCode   int
	Body         string
	Success      bool
	IsTokenError bool

	// Parsed data from successful response
	OrderID     string
	TotalAmount int64
	Currency    string
	Wallet      string

	// Transaction information
	TransactionSent   bool
	TransactionResult *TransactionResult
}

// BuyStickers performs a sticker purchase request and returns raw response
func (c *HTTPClient) BuyStickers(authToken string, collection, character int, currency string, count int) (*BuyStickersResponse, error) {
	// Form URL with parameters
	url := fmt.Sprintf("https://api.stickerdom.store/api/v1/shop/buy/crypto?collection=%d&character=%d&currency=%s&count=%d",
		collection, character, currency, count)

	// Create request
	req, err := fhttp.NewRequest("POST", url, nil)
	if err != nil {
		return nil, fmt.Errorf("error creating request: %v", err)
	}
	// Set headers as in example
	headers := map[string]string{
		"accept":             "application/json",
		"accept-language":    "ru-RU,ru;q=0.9,en-US;q=0.8,en;q=0.7",
		"authorization":      fmt.Sprintf("Bearer %s", authToken),
		"cache-control":      "no-cache",
		"pragma":             "no-cache",
		"priority":           "u=1, i",
		"sec-ch-ua":          `"Chromium";v="136", "Google Chrome";v="136", "Not.A/Brand";v="99"`,
		"sec-ch-ua-mobile":   "?0",
		"sec-ch-ua-platform": `"macOS"`,
		"sec-fetch-dest":     "empty",
		"sec-fetch-mode":     "cors",
		"sec-fetch-site":     "same-site",
		"User-Agent":         "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/136.0.0.0 Safari/537.36",
	}

	for key, value := range headers {
		req.Header.Set(key, value)
	}

	// Execute request
	resp, err := c.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("error executing request: %v", err)
	}
	defer resp.Body.Close()

	// Read response
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("error reading response: %v", err)
	}

	bodyStr := string(body)

	// Determine request success
	success := resp.StatusCode >= 200 && resp.StatusCode < 300

	// Check for token error
	isTokenError := resp.StatusCode == 401 || resp.StatusCode == 403 ||
		strings.Contains(bodyStr, "invalid_auth_token") ||
		strings.Contains(bodyStr, "unauthorized")

	// Additional check through JSON parsing
	if !isTokenError {
		var errorResp APIErrorResponse
		if err := json.Unmarshal(body, &errorResp); err == nil {
			if !errorResp.OK && errorResp.ErrorCode == "invalid_auth_token" {
				isTokenError = true
			}
		}
	}

	result := &BuyStickersResponse{
		StatusCode:   resp.StatusCode,
		Body:         bodyStr,
		Success:      success,
		IsTokenError: isTokenError,
	}

	// Parse JSON if request is successful
	if success {
		var apiResp APIResponse
		if err := json.Unmarshal(body, &apiResp); err == nil && apiResp.OK {
			result.OrderID = apiResp.Data.OrderID
			result.TotalAmount = apiResp.Data.TotalAmount
			result.Currency = apiResp.Data.Currency
			result.Wallet = apiResp.Data.Wallet
		}
	}

	return result, nil
}

// BuyStickersAndPay buys stickers and sends TON transaction
func (c *HTTPClient) BuyStickersAndPay(authToken string, collection, character int, currency string, count int, seedPhrase string, testMode bool, testAddress string) (*BuyStickersResponse, error) {
	return c.BuyStickersAndPayWithProxy(authToken, collection, character, currency, count, seedPhrase, testMode, testAddress, false, "")
}

// BuyStickersAndPayWithProxy buys stickers and sends TON transaction with proxy support
func (c *HTTPClient) BuyStickersAndPayWithProxy(authToken string, collection, character int, currency string, count int, seedPhrase string, testMode bool, testAddress string, useProxy bool, proxyURL string) (*BuyStickersResponse, error) {
	// First buy stickers
	response, err := c.BuyStickers(authToken, collection, character, currency, count)
	if err != nil {
		return nil, fmt.Errorf("error buying stickers: %v", err)
	}

	// If purchase is not successful, return response as is
	if !response.Success || response.OrderID == "" {
		return response, nil
	}

	// Create TON client with proxy support
	tonClient, err := NewTONClientWithProxy(seedPhrase, useProxy, proxyURL)
	if err != nil {
		return response, fmt.Errorf("error creating TON client: %v", err)
	}

	// Send TON transaction
	ctx := context.Background()

	// Add a small fee to the amount (approximately 0.25 TON)
	amountWithFee := response.TotalAmount + 250000000 // add 0.25 TON for fee

	targetWallet := response.Wallet
	if testMode && testAddress != "" {
		targetWallet = testAddress
	}

	txResult, err := tonClient.SendTON(ctx, targetWallet, amountWithFee, response.OrderID, testMode, testAddress)
	if err != nil {
		// Even if transaction is not sent, return transaction attempt information
		if txResult != nil {
			response.TransactionSent = false
			response.TransactionResult = txResult
		}
		return response, fmt.Errorf("error sending TON transaction: %v", err)
	}

	// Transaction successfully sent
	response.TransactionSent = true
	response.TransactionResult = txResult

	return response, nil
}

// NewForAccount creates HTTP client with account-specific proxy settings
func NewForAccount(useProxy bool, proxyURL string) (*HTTPClient, error) {
	if useProxy && proxyURL != "" {
		return NewWithProxy(proxyURL)
	}
	return New(), nil
}
