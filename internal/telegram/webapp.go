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

// WebAppService сервис для работы с Telegram Web App
type WebAppService struct {
	api         *tg.Client
	botUsername string             // имя бота, через который получаем токен
	webAppURL   string             // URL веб-приложения
	httpClient  *client.HTTPClient // HTTP клиент для запросов
}

// NewWebAppService создает новый сервис Web App
func NewWebAppService(api *tg.Client, botUsername, webAppURL string) *WebAppService {
	return &WebAppService{
		api:         api,
		botUsername: botUsername,
		webAppURL:   webAppURL,
		httpClient:  client.New(), // используем существующий HTTP клиент
	}
}

// GetBearerTokenFromWebApp получает Bearer токен через Web App
func (w *WebAppService) GetBearerTokenFromWebApp(ctx context.Context, userID int64) (string, error) {
	log.Printf("🌐 Запрос Bearer токена через Web App для бота: %s", w.botUsername)

	// 1. Находим бота
	bot, err := w.findBot(ctx)
	if err != nil {
		return "", fmt.Errorf("поиск бота: %w", err)
	}

	// 2. Запрашиваем Web App
	webAppData, err := w.requestWebApp(ctx, bot, userID)
	if err != nil {
		return "", fmt.Errorf("запрос Web App: %w", err)
	}

	// 3. Извлекаем Bearer токен из данных Web App
	token, err := w.extractBearerToken(webAppData)
	if err != nil {
		return "", fmt.Errorf("извлечение Bearer токена: %w", err)
	}

	log.Printf("✅ Bearer токен получен через Web App: %s", maskToken(token))
	return token, nil
}

// GetBearerTokenFromBot получает Bearer токен отправив команду боту
func (w *WebAppService) GetBearerTokenFromBot(ctx context.Context, userID int64) (string, error) {
	log.Printf("🤖 Запрос Bearer токена через команду боту: %s", w.botUsername)

	// 1. Находим бота
	bot, err := w.findBot(ctx)
	if err != nil {
		return "", fmt.Errorf("поиск бота: %w", err)
	}

	// 2. Отправляем команду /start или /token боту
	token, err := w.sendTokenCommand(ctx, bot, userID)
	if err != nil {
		return "", fmt.Errorf("отправка команды боту: %w", err)
	}

	log.Printf("✅ Bearer токен получен от бота: %s", maskToken(token))
	return token, nil
}

// findBot находит бота по username
func (w *WebAppService) findBot(ctx context.Context) (*tg.User, error) {
	// Резолвим username бота
	resolved, err := w.api.ContactsResolveUsername(ctx, &tg.ContactsResolveUsernameRequest{
		Username: w.botUsername,
	})
	if err != nil {
		return nil, fmt.Errorf("резолв username %s: %w", w.botUsername, err)
	}

	// Ищем пользователя-бота
	for _, user := range resolved.Users {
		if u, ok := user.(*tg.User); ok && u.Bot {
			return u, nil
		}
	}

	return nil, fmt.Errorf("бот %s не найден", w.botUsername)
}

// requestWebApp запрашивает Web App у бота
func (w *WebAppService) requestWebApp(ctx context.Context, bot *tg.User, userID int64) (string, error) {
	// Создаем input peer для бота
	inputPeer := &tg.InputPeerUser{
		UserID:     bot.ID,
		AccessHash: bot.AccessHash,
	}

	// Создаем input user для бота
	inputUser := &tg.InputUser{
		UserID:     bot.ID,
		AccessHash: bot.AccessHash,
	}

	// Запрашиваем Web App
	webView, err := w.api.MessagesRequestWebView(ctx, &tg.MessagesRequestWebViewRequest{
		Peer:     inputPeer,
		Bot:      inputUser,
		URL:      w.webAppURL,
		Platform: "web",
	})
	if err != nil {
		return "", fmt.Errorf("запрос Web App: %w", err)
	}

	log.Printf("🔗 Web App URL: %s", webView.URL)

	// Возвращаем полные данные URL для дальнейшей обработки
	return webView.URL, nil
}

// sendTokenCommand отправляет команду боту для получения токена
func (w *WebAppService) sendTokenCommand(ctx context.Context, bot *tg.User, userID int64) (string, error) {
	// Создаем input peer для бота
	inputPeer := &tg.InputPeerUser{
		UserID:     bot.ID,
		AccessHash: bot.AccessHash,
	}

	// Отправляем команду /token или /start
	_, err := w.api.MessagesSendMessage(ctx, &tg.MessagesSendMessageRequest{
		Peer:    inputPeer,
		Message: "/token",
	})
	if err != nil {
		return "", fmt.Errorf("отправка команды: %w", err)
	}

	log.Printf("📤 Команда /token отправлена боту")

	// Ждем ответа от бота (это упрощенная версия)
	// В реальности нужно настроить обработчик сообщений
	time.Sleep(2 * time.Second)

	// Получаем последние сообщения
	messages, err := w.api.MessagesGetHistory(ctx, &tg.MessagesGetHistoryRequest{
		Peer:  inputPeer,
		Limit: 10,
	})
	if err != nil {
		return "", fmt.Errorf("получение истории: %w", err)
	}

	// Ищем токен в сообщениях
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

	return "", fmt.Errorf("токен не найден в ответах бота")
}

// extractBearerToken извлекает Bearer токен из данных Web App
func (w *WebAppService) extractBearerToken(webAppURL string) (string, error) {
	log.Printf("🔍 Анализ Web App URL: %s", webAppURL)

	// Парсим URL и извлекаем данные
	parsedURL, err := url.Parse(webAppURL)
	if err != nil {
		return "", fmt.Errorf("парсинг URL: %w", err)
	}

	// Проверяем обычные параметры запроса
	queryParams := parsedURL.Query()

	// 1. Проверяем прямые токены в параметрах
	tokenParams := []string{"token", "auth_token", "bearer", "access_token", "jwt"}
	for _, param := range tokenParams {
		if token := queryParams.Get(param); token != "" {
			log.Printf("✅ Найден токен в параметре %s", param)
			return token, nil
		}
	}

	// 2. Проверяем токен в hash части URL (после #)
	if fragment := parsedURL.Fragment; fragment != "" {
		if token := extractTokenFromFragment(fragment); token != "" {
			log.Printf("✅ Найден токен во fragment")
			return token, nil
		}
	}

	// 3. Извлекаем tgWebAppData/initData из URL
	initData := queryParams.Get("tgWebAppData")
	if initData == "" {
		initData = queryParams.Get("initData")
	}

	if initData != "" {
		log.Printf("🔍 Найден initData, отправляем на API для получения токена")
		return w.requestTokenWithInitData(initData)
	}

	// 4. Если это прямая ссылка на Web App, пробуем загрузить её
	if strings.Contains(webAppURL, "tgWebAppData=") || strings.Contains(webAppURL, "initData=") {
		return w.extractInitDataFromURL(webAppURL)
	}

	// 5. Последняя попытка - запрос к API приложения
	return w.requestTokenFromWebAppAPI(webAppURL)
}

// extractInitDataFromURL извлекает initData из URL
func (w *WebAppService) extractInitDataFromURL(webAppURL string) (string, error) {
	// Ищем tgWebAppData или initData в URL
	re := regexp.MustCompile(`(?:tgWebAppData|initData)=([^&\s#]+)`)
	matches := re.FindStringSubmatch(webAppURL)

	if len(matches) < 2 {
		return "", fmt.Errorf("initData не найден в URL")
	}

	initData, err := url.QueryUnescape(matches[1])
	if err != nil {
		return "", fmt.Errorf("ошибка декодирования initData: %w", err)
	}

	log.Printf("🔍 Извлечен initData: %s...", initData[:min(50, len(initData))])

	return w.requestTokenWithInitData(initData)
}

// requestTokenWithInitData отправляет initData на API для получения токена
func (w *WebAppService) requestTokenWithInitData(initData string) (string, error) {
	// Здесь должен быть HTTP запрос к вашему API
	// который принимает initData и возвращает Bearer токен

	log.Printf("📤 Отправка initData на API приложения")

	/* Пример реализации:

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
		return "", fmt.Errorf("запрос к API: %w", err)
	}
	defer resp.Body.Close()

	var tokenResp TokenResponse
	if err := json.NewDecoder(resp.Body).Decode(&tokenResp); err != nil {
		return "", fmt.Errorf("декодирование ответа: %w", err)
	}

	if tokenResp.Error != "" {
		return "", fmt.Errorf("ошибка API: %s", tokenResp.Error)
	}

	return tokenResp.Token, nil
	*/

	// Для демонстрации - создаем токен на основе initData
	// В реальности здесь должен быть вызов вашего API!
	token := fmt.Sprintf("demo_token_%x", initData[:min(8, len(initData))])
	log.Printf("⚠️  ДЕМО: Создан тестовый токен: %s", maskToken(token))
	log.Printf("⚠️  ВНИМАНИЕ: Реализуйте requestTokenWithInitData для вашего API!")

	return token, nil
}

// requestTokenFromWebAppAPI делает дополнительный запрос к API приложения
func (w *WebAppService) requestTokenFromWebAppAPI(webAppURL string) (string, error) {
	// Эта функция вызывается если не найден initData
	// Можете реализовать альтернативную логику получения токена

	log.Printf("⚠️  Web App URL не содержит initData или прямого токена: %s", webAppURL)
	log.Printf("⚠️  Попробуйте:")
	log.Printf("    1. Проверить правильность bot_username")
	log.Printf("    2. Убедиться что бот имеет Web App")
	log.Printf("    3. Проверить web_app_url в конфигурации")

	return "", fmt.Errorf("не удалось извлечь токен из Web App URL")
}

// min возвращает минимальное значение
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// extractTokenFromFragment извлекает токен из fragment части URL
func extractTokenFromFragment(fragment string) string {
	// Парсим fragment как query string
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

// extractTokenFromMessage извлекает токен из текста сообщения
func extractTokenFromMessage(message string) string {
	// Регулярные выражения для поиска токенов
	tokenPatterns := []string{
		`(?i)token[:\s]+([A-Za-z0-9_\-\.]+)`,
		`(?i)bearer[:\s]+([A-Za-z0-9_\-\.]+)`,
		`(?i)auth[:\s]+([A-Za-z0-9_\-\.]+)`,
		`([A-Za-z0-9_\-\.]{32,})`, // Длинные строки (возможные токены)
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

// maskToken маскирует токен для безопасного логирования
func maskToken(token string) string {
	if len(token) <= 8 {
		return strings.Repeat("*", len(token))
	}
	return token[:4] + strings.Repeat("*", len(token)-8) + token[len(token)-4:]
}

// GetAuthData получает auth data из Telegram Web App (аналог Python функции)
func (w *WebAppService) GetAuthData(ctx context.Context, botTag, webAppURL string) (*client.TelegramAuthResponse, error) {
	log.Printf("🔍 Получение auth data для бота: %s", botTag)

	// 1. Находим бота
	bot, err := w.findBotByTag(ctx, botTag)
	if err != nil {
		log.Printf("❌ Ошибка поиска бота: %v", err)
		return &client.TelegramAuthResponse{
			Status:      "ERROR",
			Description: "Bot not found",
			Data:        err,
		}, err
	}

	// 2. Запрашиваем Web App
	webAppData, err := w.requestWebAppData(ctx, bot, webAppURL)
	if err != nil {
		log.Printf("❌ Ошибка получения Web App данных: %v", err)
		return &client.TelegramAuthResponse{
			Status:      "ERROR",
			Description: "Failed to get Web App data",
			Data:        err,
		}, err
	}

	log.Printf("✅ Auth data получен успешно")
	return &client.TelegramAuthResponse{
		Status:      "SUCCESS",
		Description: "OK",
		Data:        webAppData,
	}, nil
}

// findBotByTag находит бота по tag (аналог resolve_peer)
func (w *WebAppService) findBotByTag(ctx context.Context, botTag string) (*tg.User, error) {
	// Убираем @ если есть
	botUsername := strings.TrimPrefix(botTag, "@")

	// Резолвим username бота
	resolved, err := w.api.ContactsResolveUsername(ctx, &tg.ContactsResolveUsernameRequest{
		Username: botUsername,
	})
	if err != nil {
		return nil, fmt.Errorf("резолв username %s: %w", botUsername, err)
	}

	// Ищем пользователя-бота
	for _, user := range resolved.Users {
		if u, ok := user.(*tg.User); ok && u.Bot {
			return u, nil
		}
	}

	return nil, fmt.Errorf("бот %s не найден", botTag)
}

// requestWebAppData запрашивает Web App данные (аналог RequestWebView)
func (w *WebAppService) requestWebAppData(ctx context.Context, bot *tg.User, webAppURL string) (*client.AuthData, error) {
	// Создаем input peer для бота
	inputPeer := &tg.InputPeerUser{
		UserID:     bot.ID,
		AccessHash: bot.AccessHash,
	}

	// Создаем input user для бота
	inputUser := &tg.InputUser{
		UserID:     bot.ID,
		AccessHash: bot.AccessHash,
	}

	// Запрашиваем Web App (аналог RequestWebView из Python)
	webView, err := w.api.MessagesRequestWebView(ctx, &tg.MessagesRequestWebViewRequest{
		Peer:        inputPeer,
		Bot:         inputUser,
		URL:         webAppURL,
		Platform:    "android", // как в Python коде
		FromBotMenu: false,     // как в Python коде
	})
	if err != nil {
		return nil, fmt.Errorf("запрос Web App: %w", err)
	}

	log.Printf("🔗 Получен Web App URL: %s", webView.URL)

	// Извлекаем tgWebAppData из URL (как в Python)
	authDataString, err := w.extractTgWebAppData(webView.URL)
	if err != nil {
		return nil, fmt.Errorf("извлечение tgWebAppData: %w", err)
	}

	// Создаем AuthData с временем истечения 45 минут (как в Python)
	expTime := time.Now().Add(45 * time.Minute)
	authData := client.NewAuthData(authDataString, expTime)

	log.Printf("📋 Auth data извлечен, истекает: %s", expTime.Format("15:04:05"))

	return authData, nil
}

// extractTgWebAppData извлекает и декодирует tgWebAppData (аналог Python unquote)
func (w *WebAppService) extractTgWebAppData(webAppURL string) (string, error) {
	// Ищем tgWebAppData в URL
	if !strings.Contains(webAppURL, "tgWebAppData=") {
		return "", fmt.Errorf("tgWebAppData не найден в URL")
	}

	// Разделяем URL и извлекаем часть с tgWebAppData
	parts := strings.Split(webAppURL, "tgWebAppData=")
	if len(parts) < 2 {
		return "", fmt.Errorf("некорректный формат URL")
	}

	// Извлекаем данные до следующего параметра
	tgWebAppData := strings.Split(parts[1], "&tgWebAppVersion")[0]

	// Декодируем URL (аналог Python unquote)
	decoded1, err := url.QueryUnescape(tgWebAppData)
	if err != nil {
		return "", fmt.Errorf("первое декодирование: %w", err)
	}

	// Второе декодирование (как в Python - двойной unquote)
	decoded2, err := url.QueryUnescape(decoded1)
	if err != nil {
		return "", fmt.Errorf("второе декодирование: %w", err)
	}

	log.Printf("🔓 Декодированные auth data: %s...", decoded2[:min(50, len(decoded2))])

	return decoded2, nil
}
