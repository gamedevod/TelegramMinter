package client

import (
	"fmt"
	"io"
	"strings"

	fhttp "github.com/bogdanfinn/fhttp"
	tls_client "github.com/bogdanfinn/tls-client"
	"github.com/bogdanfinn/tls-client/profiles"
)

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
	StatusCode int
	Body       string
	Success    bool
	IsTokenError bool
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
	fmt.Println(authToken)
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
	isTokenError := strings.Contains(bodyStr, "invalid_auth_token") || 
					strings.Contains(bodyStr, "unauthorized") ||
					resp.StatusCode == 401

	return &BuyStickersResponse{
		StatusCode:   resp.StatusCode,
		Body:         bodyStr,
		Success:      success,
		IsTokenError: isTokenError,
	}, nil
} 