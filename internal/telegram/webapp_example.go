package telegram

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

// Пример реализации запроса к вашему API для получения Bearer токена

// InitDataRequest запрос с initData
type InitDataRequest struct {
	InitData string `json:"init_data"`
}

// TokenResponse ответ с токеном
type TokenResponse struct {
	Token     string `json:"token"`
	ExpiresAt string `json:"expires_at,omitempty"`
	Error     string `json:"error,omitempty"`
}

// requestTokenFromAPI отправляет initData на ваш API и получает Bearer токен
func requestTokenFromAPI(initData, apiURL string) (string, error) {
	// Подготавливаем запрос
	reqBody := InitDataRequest{
		InitData: initData,
	}

	jsonData, err := json.Marshal(reqBody)
	if err != nil {
		return "", fmt.Errorf("кодирование JSON: %w", err)
	}

	// Создаем HTTP клиент с таймаутом
	client := &http.Client{
		Timeout: 30 * time.Second,
	}

	// Отправляем POST запрос
	resp, err := client.Post(apiURL, "application/json", bytes.NewBuffer(jsonData))
	if err != nil {
		return "", fmt.Errorf("HTTP запрос: %w", err)
	}
	defer resp.Body.Close()

	// Проверяем статус ответа
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("API вернул статус %d", resp.StatusCode)
	}

	// Декодируем ответ
	var tokenResp TokenResponse
	if err := json.NewDecoder(resp.Body).Decode(&tokenResp); err != nil {
		return "", fmt.Errorf("декодирование ответа: %w", err)
	}

	// Проверяем на ошибки
	if tokenResp.Error != "" {
		return "", fmt.Errorf("ошибка API: %s", tokenResp.Error)
	}

	if tokenResp.Token == "" {
		return "", fmt.Errorf("API не вернул токен")
	}

	return tokenResp.Token, nil
}

// Пример использования:
/*
func (w *WebAppService) requestTokenWithInitData(initData string) (string, error) {
	// URL вашего API endpoint
	apiURL := "https://your-api.com/auth/telegram-webapp"

	return requestTokenFromAPI(initData, apiURL)
}
*/
