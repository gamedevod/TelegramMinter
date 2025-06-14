package client

import (
	"encoding/json"
	"fmt"
	"io"
	"time"
)

// AuthData structure for storing authentication data
type AuthData struct {
	Data string    `json:"data"`
	Exp  time.Time `json:"exp"`
}

// NewAuthData creates new AuthData with data and expiration time
func NewAuthData(data string, exp time.Time) *AuthData {
	return &AuthData{
		Data: data,
		Exp:  exp,
	}
}

// IsValid checks if authentication data is valid
func (ad *AuthData) IsValid() bool {
	return time.Now().Before(ad.Exp)
}

// AuthResponse response from authentication API
type AuthResponse struct {
	OK   bool   `json:"ok"`
	Data string `json:"data"` // Bearer token
}

// TelegramAuthResponse authentication response structure
type TelegramAuthResponse struct {
	Status      string      `json:"status"`
	Description string      `json:"description"`
	Data        interface{} `json:"data"`
}

// AuthenticateWithTelegramData performs authentication by sending authData to API
func (c *HTTPClient) AuthenticateWithTelegramData(apiURL string, authData *AuthData) (*TelegramAuthResponse, error) {
	// Log data being sent
	fmt.Printf("🔍 Data being sent: %s\n", authData.Data[:min(100, len(authData.Data))])

	// Send data as raw text (as in curl)
	rawData := authData.Data

	headers := map[string]string{
		"User-Agent":      "Mozilla/5.0 (Windows NT 10.0; Win64; x64; rv:139.0) Gecko/20100101 Firefox/139.0",
		"Accept":          "application/json",
		"Accept-Language": "ru-RU,ru;q=0.8,en-US;q=0.5,en;q=0.3",
		"Accept-Encoding": "gzip, deflate, br, zstd",
		"Referer":         "https://stickerdom.store/",
		"Content-Type":    "text/plain;charset=UTF-8",
		"Origin":          "https://stickerdom.store",
		"Connection":      "keep-alive",
		"Sec-Fetch-Dest":  "empty",
		"Sec-Fetch-Mode":  "cors",
		"Sec-Fetch-Site":  "same-site",
		"Priority":        "u=4",
		"TE":              "trailers",
	}

	// Send POST request to correct endpoint
	resp, err := c.Post(fmt.Sprintf("%s/auth", apiURL), rawData, headers)
	if err != nil {
		return &TelegramAuthResponse{
			Status:      "ERROR",
			Description: "HTTP request failed",
		}, err
	}
	defer resp.Body.Close()

	// Read response
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return &TelegramAuthResponse{
			Status:      "ERROR",
			Description: "Failed to read response",
		}, err
	}

	// Log response for debugging
	fmt.Printf("🔍 API Response: %s\n", string(body))

	if resp.StatusCode == 200 {
		var authResp AuthResponse
		if err := json.Unmarshal(body, &authResp); err != nil {
			return &TelegramAuthResponse{
				Status:      "ERROR",
				Description: "Invalid response format",
			}, err
		}

		if authResp.OK {
			return &TelegramAuthResponse{
				Status:      "SUCCESS",
				Description: "OK",
				Data:        authResp.Data,
			}, nil
		} else {
			return &TelegramAuthResponse{
				Status:      "ERROR",
				Description: "Session expired",
			}, nil
		}
	} else {
		return &TelegramAuthResponse{
			Status:      "ERROR",
			Description: fmt.Sprintf("API returned status %d: %s", resp.StatusCode, string(body)),
		}, fmt.Errorf("API returned status %d", resp.StatusCode)
	}
}

// min returns minimum value
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
