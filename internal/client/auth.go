package client

import (
	"encoding/json"
	"fmt"
	"io"
	"time"
)

// AuthData структура для хранения данных авторизации
type AuthData struct {
	Data string    `json:"data"`
	Exp  time.Time `json:"exp"`
}

// NewAuthData создает новый AuthData с данными и временем истечения
func NewAuthData(data string, exp time.Time) *AuthData {
	return &AuthData{
		Data: data,
		Exp:  exp,
	}
}

// IsValid проверяет, действительны ли данные авторизации
func (ad *AuthData) IsValid() bool {
	return time.Now().Before(ad.Exp)
}

// AuthResponse ответ от API авторизации
type AuthResponse struct {
	OK   bool   `json:"ok"`
	Data string `json:"data"` // Bearer токен
}

// TelegramAuthResponse структура ответа авторизации
type TelegramAuthResponse struct {
	Status      string      `json:"status"`
	Description string      `json:"description"`
	Data        interface{} `json:"data"`
}

// AuthenticateWithTelegramData выполняет авторизацию отправив authData на API
func (c *HTTPClient) AuthenticateWithTelegramData(apiURL string, authData *AuthData) (*TelegramAuthResponse, error) {
	// Логируем данные которые отправляем
	fmt.Printf("🔍 Отправляемые данные: %s\n", authData.Data[:min(100, len(authData.Data))])

	// Отправляем данные как raw text (как в curl)
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

	// Отправляем POST запрос на правильный endpoint
	resp, err := c.Post(fmt.Sprintf("%s/auth", apiURL), rawData, headers)
	if err != nil {
		return &TelegramAuthResponse{
			Status:      "ERROR",
			Description: "HTTP request failed",
		}, err
	}
	defer resp.Body.Close()

	// Читаем ответ
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return &TelegramAuthResponse{
			Status:      "ERROR",
			Description: "Failed to read response",
		}, err
	}

	// Логируем ответ для отладки
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

// min возвращает минимальное значение
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
