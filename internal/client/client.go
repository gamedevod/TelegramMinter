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

// APIResponse структура для успешного ответа от API
type APIResponse struct {
	OK   bool `json:"ok"`
	Data struct {
		OrderID     string `json:"order_id"`
		TotalAmount int64  `json:"total_amount"`
		Currency    string `json:"currency"`
		Wallet      string `json:"wallet"`
	} `json:"data"`
}

// APIErrorResponse структура для ошибочного ответа от API
type APIErrorResponse struct {
	OK        bool   `json:"ok"`
	ErrorCode string `json:"errorCode"`
}

// HTTPClient обертка для tls-client
type HTTPClient struct {
	client tls_client.HttpClient
}

// New создает новый HTTP клиент
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
		panic(fmt.Sprintf("Ошибка создания HTTP клиента: %v", err))
	}

	return &HTTPClient{
		client: client,
	}
}

// Get выполняет GET запрос
func (c *HTTPClient) Get(url string, headers map[string]string) (*fhttp.Response, error) {
	req, err := fhttp.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}

	// Добавляем заголовки
	for key, value := range headers {
		req.Header.Set(key, value)
	}

	return c.client.Do(req)
}

// Post выполняет POST запрос
func (c *HTTPClient) Post(url string, body string, headers map[string]string) (*fhttp.Response, error) {
	req, err := fhttp.NewRequest("POST", url, strings.NewReader(body))
	if err != nil {
		return nil, err
	}

	// Добавляем заголовки
	for key, value := range headers {
		req.Header.Set(key, value)
	}

	return c.client.Do(req)
}

// BuyStickersResponse структура ответа для покупки стикеров
type BuyStickersResponse struct {
	StatusCode   int
	Body         string
	Success      bool
	IsTokenError bool

	// Распарсенные данные из успешного ответа
	OrderID     string
	TotalAmount int64
	Currency    string
	Wallet      string

	// Информация о транзакции
	TransactionSent   bool
	TransactionResult *TransactionResult
}

// BuyStickers выполняет запрос на покупку стикеров и возвращает сырой ответ
func (c *HTTPClient) BuyStickers(authToken string, collection, character int, currency string, count int) (*BuyStickersResponse, error) {
	// Формируем URL с параметрами
	url := fmt.Sprintf("https://api.stickerdom.store/api/v1/shop/buy/crypto?collection=%d&character=%d&currency=%s&count=%d",
		collection, character, currency, count)

	// Создаем запрос
	req, err := fhttp.NewRequest("POST", url, nil)
	if err != nil {
		return nil, fmt.Errorf("ошибка создания запроса: %v", err)
	}
	// Устанавливаем заголовки как в примере
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

	// Выполняем запрос
	resp, err := c.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("ошибка выполнения запроса: %v", err)
	}
	defer resp.Body.Close()

	// Читаем ответ
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("ошибка чтения ответа: %v", err)
	}

	bodyStr := string(body)

	// Определяем успешность запроса
	success := resp.StatusCode >= 200 && resp.StatusCode < 300

	// Проверяем на ошибку токена
	isTokenError := resp.StatusCode == 401 || resp.StatusCode == 403 ||
		strings.Contains(bodyStr, "invalid_auth_token") ||
		strings.Contains(bodyStr, "unauthorized")

	// Дополнительная проверка через JSON парсинг
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

	// Парсим JSON если запрос успешен
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

// BuyStickersAndPay покупает стикеры и отправляет TON транзакцию
func (c *HTTPClient) BuyStickersAndPay(authToken string, collection, character int, currency string, count int, seedPhrase string, testMode bool, testAddress string) (*BuyStickersResponse, error) {
	// Сначала покупаем стикеры
	response, err := c.BuyStickers(authToken, collection, character, currency, count)
	if err != nil {
		return nil, fmt.Errorf("ошибка покупки стикеров: %v", err)
	}

	// Если покупка не успешна, возвращаем ответ как есть
	if !response.Success || response.OrderID == "" {
		return response, nil
	}

	// Создаем TON клиент
	tonClient, err := NewTONClient(seedPhrase)
	if err != nil {
		return response, fmt.Errorf("ошибка создания TON клиента: %v", err)
	}

	// Отправляем TON транзакцию
	ctx := context.Background()

	// Добавляем небольшую комиссию к сумме (примерно 0.25 TON)
	amountWithFee := response.TotalAmount + 250000000 // добавляем 0.25 TON на комиссию

	targetWallet := response.Wallet
	if testMode && testAddress != "" {
		targetWallet = testAddress
	}

	txResult, err := tonClient.SendTON(ctx, targetWallet, amountWithFee, response.OrderID, testMode, testAddress)
	if err != nil {
		// Даже если транзакция не отправлена, возвращаем информацию о попытке
		if txResult != nil {
			response.TransactionSent = false
			response.TransactionResult = txResult
		}
		return response, fmt.Errorf("ошибка отправки TON транзакции: %v", err)
	}

	// Транзакция успешно отправлена
	response.TransactionSent = true
	response.TransactionResult = txResult

	return response, nil
}
