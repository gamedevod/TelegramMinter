package monitor

import (
	"encoding/json"
	"fmt"
	"io"
	"stickersbot/internal/client"
	"strings"
)

// APIClient клиент для работы с API коллекций
type APIClient struct {
	httpClient *client.HTTPClient
	baseURL    string
}

// NewAPIClient создает новый API клиент
func NewAPIClient(httpClient *client.HTTPClient) *APIClient {
	return &APIClient{
		httpClient: httpClient,
		baseURL:    "https://api.stickerdom.store/api/v1",
	}
}

// APIResponse структура для проверки ошибок токенов
type APIResponse struct {
	OK        bool   `json:"ok"`
	ErrorCode string `json:"errorCode"`
}

// TokenError ошибка токена
type TokenError struct {
	StatusCode int
	Body       string
}

func (e *TokenError) Error() string {
	return fmt.Sprintf("ошибка токена: статус %d, тело: %s", e.StatusCode, e.Body)
}

// isTokenError проверяет, является ли ответ ошибкой токена
func (a *APIClient) isTokenError(statusCode int, bodyStr string) bool {
	// Проверяем на ошибку токена
	isTokenError := statusCode == 401 || statusCode == 403 ||
		strings.Contains(bodyStr, "invalid_auth_token") ||
		strings.Contains(bodyStr, "unauthorized")

	// Дополнительная проверка через JSON парсинг
	if !isTokenError {
		var errorResp APIResponse
		if err := json.Unmarshal([]byte(bodyStr), &errorResp); err == nil {
			if !errorResp.OK && errorResp.ErrorCode == "invalid_auth_token" {
				isTokenError = true
			}
		}
	}

	return isTokenError
}

// GetCollections получает список коллекций
func (a *APIClient) GetCollections(authToken string) (*CollectionsResponse, error) {
	url := fmt.Sprintf("%s/collections", a.baseURL)

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

	resp, err := a.httpClient.Get(url, headers)
	if err != nil {
		return nil, fmt.Errorf("ошибка GET запроса: %v", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("ошибка чтения ответа: %v", err)
	}

	// Проверяем на ошибку токена
	if a.isTokenError(resp.StatusCode, string(body)) {
		return nil, &TokenError{
			StatusCode: resp.StatusCode,
			Body:       string(body),
		}
	}

	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("неуспешный статус код: %d", resp.StatusCode)
	}

	var response CollectionsResponse
	if err := json.Unmarshal(body, &response); err != nil {
		return nil, fmt.Errorf("ошибка парсинга JSON: %v", err)
	}

	if !response.OK {
		return nil, fmt.Errorf("API вернул ok=false")
	}

	return &response, nil
}

// GetCollectionDetails получает детали коллекции по ID
func (a *APIClient) GetCollectionDetails(authToken string, collectionID int) (*CollectionDetailsResponse, error) {
	url := fmt.Sprintf("%s/collection/%d", a.baseURL, collectionID)

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

	resp, err := a.httpClient.Get(url, headers)
	if err != nil {
		return nil, fmt.Errorf("ошибка GET запроса: %v", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("ошибка чтения ответа: %v", err)
	}

	// Проверяем на ошибку токена
	if a.isTokenError(resp.StatusCode, string(body)) {
		return nil, &TokenError{
			StatusCode: resp.StatusCode,
			Body:       string(body),
		}
	}

	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("неуспешный статус код: %d", resp.StatusCode)
	}

	var response CollectionDetailsResponse
	if err := json.Unmarshal(body, &response); err != nil {
		return nil, fmt.Errorf("ошибка парсинга JSON: %v", err)
	}

	if !response.OK {
		return nil, fmt.Errorf("API вернул ok=false")
	}

	return &response, nil
}
