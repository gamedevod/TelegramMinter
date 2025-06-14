package telegram

import (
	"bufio"
	"context"
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	"stickersbot/internal/client"

	"github.com/gotd/td/session"
	"github.com/gotd/td/telegram"
	"github.com/gotd/td/telegram/auth"
	"github.com/gotd/td/tg"
)

// AuthService структура для авторизации в Telegram
type AuthService struct {
	APIId       int
	APIHash     string
	PhoneNumber string
	SessionFile string
	BotUsername string // Username бота для получения токена
	WebAppURL   string // URL Web App
	TokenAPIURL string // URL API для получения Bearer токена
	client      *telegram.Client
}

// NewAuthService создает новый сервис авторизации
func NewAuthService(apiId int, apiHash, phoneNumber, sessionFile, botUsername, webAppURL, tokenAPIURL string) *AuthService {
	return &AuthService{
		APIId:       apiId,
		APIHash:     apiHash,
		PhoneNumber: phoneNumber,
		SessionFile: sessionFile,
		BotUsername: botUsername,
		WebAppURL:   webAppURL,
		TokenAPIURL: tokenAPIURL,
	}
}

// AuthorizeAndGetToken авторизуется в Telegram и получает Bearer токен
func (a *AuthService) AuthorizeAndGetToken(ctx context.Context) (string, error) {
	// Создаем сессию из файла
	sessionStorage := &session.FileStorage{
		Path: a.SessionFile,
	}

	// Создаем клиент
	a.client = telegram.NewClient(a.APIId, a.APIHash, telegram.Options{
		SessionStorage: sessionStorage,
	})

	var bearerToken string

	// Запускаем клиент
	err := a.client.Run(ctx, func(ctx context.Context) error {
		// Проверяем авторизацию
		status, err := a.client.Auth().Status(ctx)
		if err != nil {
			return fmt.Errorf("проверка статуса авторизации: %w", err)
		}

		if !status.Authorized {
			// Нужна авторизация
			log.Printf("🔐 Авторизация для номера: %s", a.PhoneNumber)

			if err := a.performAuth(ctx); err != nil {
				return fmt.Errorf("авторизация: %w", err)
			}
		} else {
			log.Printf("✅ Уже авторизован для номера: %s", a.PhoneNumber)
		}

		// Получаем Bearer токен через Web App авторизацию
		token, err := a.getBearerToken(ctx)
		if err != nil {
			return fmt.Errorf("получение Bearer токена: %w", err)
		}

		bearerToken = token
		return nil
	})

	if err != nil {
		return "", err
	}

	return bearerToken, nil
}

// performAuth выполняет авторизацию по номеру телефона
func (a *AuthService) performAuth(ctx context.Context) error {
	flow := auth.NewFlow(
		auth.Constant(
			a.PhoneNumber,
			"", // пароль оставляем пустым
			auth.CodeAuthenticatorFunc(a.codePrompt),
		),
		auth.SendCodeOptions{},
	)

	return a.client.Auth().IfNecessary(ctx, flow)
}

// codePrompt запрашивает код подтверждения у пользователя
func (a *AuthService) codePrompt(ctx context.Context, sentCode *tg.AuthSentCode) (string, error) {
	fmt.Printf("📱 Код подтверждения отправлен на номер: %s\n", a.PhoneNumber)
	fmt.Print("Введите код: ")

	reader := bufio.NewReader(os.Stdin)
	code, err := reader.ReadString('\n')
	if err != nil {
		return "", err
	}

	return strings.TrimSpace(code), nil
}

// getBearerToken получает Bearer токен для Web App
func (a *AuthService) getBearerToken(ctx context.Context) (string, error) {
	api := a.client.API()

	// Получаем информацию о себе
	self, err := api.UsersGetFullUser(ctx, &tg.InputUserSelf{})
	if err != nil {
		return "", fmt.Errorf("получение информации о пользователе: %w", err)
	}

	user := self.Users[0].(*tg.User)
	log.Printf("👤 Авторизован как: %s %s (@%s)",
		user.FirstName,
		user.LastName,
		user.Username)

	// Здесь нужно получить Bearer токен для конкретного бота/приложения
	// Это зависит от того, как ваше приложение получает токен
	// Например, через встроенный Web App или API бота

	// Для демонстрации - генерируем токен на основе данных пользователя
	// В реальности здесь должен быть вызов к API вашего приложения
	token, err := a.generateBearerToken(ctx, user)
	if err != nil {
		return "", fmt.Errorf("генерация Bearer токена: %w", err)
	}

	return token, nil
}

// generateBearerToken генерирует Bearer токен
// Использует точно такую же логику как в Python коде
func (a *AuthService) generateBearerToken(ctx context.Context, user *tg.User) (string, error) {
	api := a.client.API()

	// Используем настройки из конфигурации или значения по умолчанию
	botUsername := a.BotUsername
	webAppURL := a.WebAppURL

	// Значения по умолчанию если не заданы в конфигурации
	if botUsername == "" {
		botUsername = "stickersbot" // замените на ваш бот
	}
	if webAppURL == "" {
		webAppURL = "https://stickers.bot/app" // замените на ваш URL
	}

	log.Printf("🔧 Используем бота: %s, Web App: %s", botUsername, webAppURL)

	// 1. Получаем auth data (аналог get_auth_data из Python)
	log.Printf("🔄 Получение auth data для бота %s...", botUsername)
	webAppService := NewWebAppService(api, botUsername, webAppURL)
	authResponse, err := webAppService.GetAuthData(ctx, botUsername, webAppURL)
	if err != nil {
		log.Printf("❌ Ошибка получения auth data: %v", err)
		log.Printf("🔄 Переход к fallback токену...")
		return a.fallbackToTempToken(user.ID)
	}

	if authResponse.Status != "SUCCESS" {
		log.Printf("❌ Auth data получить не удалось: %s", authResponse.Description)
		log.Printf("🔄 Переход к fallback токену...")
		return a.fallbackToTempToken(user.ID)
	}

	log.Printf("✅ Auth data успешно получен")

	authData, ok := authResponse.Data.(*client.AuthData)
	if !ok {
		log.Printf("⚠️  Неверный формат auth data")
		return a.fallbackToTempToken(user.ID)
	}

	// Проверяем что auth data действительны
	if !authData.IsValid() {
		log.Printf("⚠️  Auth data истек")
		return a.fallbackToTempToken(user.ID)
	}

	// 2. Отправляем auth data на API для получения Bearer токена (аналог auth из Python)
	apiURL := a.TokenAPIURL
	if apiURL == "" {
		apiURL = "https://api.stickerdom.store" // исправляем на правильный API
	}

	log.Printf("🌐 Используем API URL: %s", apiURL)

	// Используем существующий HTTPClient
	httpClient := client.New()

	// Отправляем auth data на API
	log.Printf("🔄 Отправка auth data на API %s...", apiURL)
	tokenResponse, err := httpClient.AuthenticateWithTelegramData(apiURL, authData)
	if err != nil {
		log.Printf("❌ Ошибка авторизации через API: %v", err)
		log.Printf("🔄 Переход к fallback токену...")
		return a.fallbackToTempToken(user.ID)
	}

	if tokenResponse.Status == "SUCCESS" {
		bearerToken := tokenResponse.Data.(string)
		log.Printf("✅ Bearer токен получен через API: %s", maskToken(bearerToken))
		return bearerToken, nil
	}

	log.Printf("❌ API авторизация не удалась: %s", tokenResponse.Description)
	log.Printf("🔄 Переход к fallback токену...")
	return a.fallbackToTempToken(user.ID)
}

// fallbackToTempToken создает временный токен если основные методы не сработали
func (a *AuthService) fallbackToTempToken(userID int64) (string, error) {
	timestamp := time.Now().Unix()
	tempToken := fmt.Sprintf("tg_token_%d_%d", userID, timestamp)

	log.Printf("🎫 Создан временный Bearer токен: %s", maskToken(tempToken))
	log.Printf("⚠️  ВНИМАНИЕ: Используется временный токен!")
	log.Printf("⚠️  Проверьте настройки: bot_username=%s, web_app_url=%s, token_api_url=%s",
		a.BotUsername, a.WebAppURL, a.TokenAPIURL)

	return tempToken, nil
}

// requestTokenFromYourAPI пример запроса токена от вашего API
func (a *AuthService) requestTokenFromYourAPI(userID int64) (string, error) {
	// Здесь должен быть HTTP запрос к вашему API
	// который принимает Telegram User ID и возвращает Bearer токен

	// Пример:
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

	return "", fmt.Errorf("метод не реализован - добавьте свою логику получения токена")
}
