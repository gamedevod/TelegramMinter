package telegram

import (
	"bufio"
	"context"
	"fmt"
	"log"
	"os"
	"strings"
	"time"

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
	client      *telegram.Client
}

// NewAuthService создает новый сервис авторизации
func NewAuthService(apiId int, apiHash, phoneNumber, sessionFile, botUsername, webAppURL string) *AuthService {
	return &AuthService{
		APIId:       apiId,
		APIHash:     apiHash,
		PhoneNumber: phoneNumber,
		SessionFile: sessionFile,
		BotUsername: botUsername,
		WebAppURL:   webAppURL,
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
// Использует реальные методы получения токена из Web App или бота
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

	// Способ 1: Попробовать получить токен через Web App
	webAppService := NewWebAppService(api, botUsername, webAppURL)
	token, err := webAppService.GetBearerTokenFromWebApp(ctx, user.ID)
	if err == nil {
		return token, nil
	}
	log.Printf("⚠️  Web App не сработал: %v", err)

	// Способ 2: Попробовать получить токен отправив команду боту
	token, err = webAppService.GetBearerTokenFromBot(ctx, user.ID)
	if err == nil {
		return token, nil
	}
	log.Printf("⚠️  Команда боту не сработала: %v", err)

	// Способ 3: Получить токен через ваш API endpoint
	token, err = a.requestTokenFromYourAPI(user.ID)
	if err == nil {
		return token, nil
	}
	log.Printf("⚠️  API endpoint не сработал: %v", err)

	// Способ 4: Для демонстрации - создаем временный токен
	timestamp := time.Now().Unix()
	tempToken := fmt.Sprintf("tg_token_%d_%d", user.ID, timestamp)

	log.Printf("🎫 Сгенерирован временный Bearer токен: %s", tempToken)
	log.Printf("⚠️  ВНИМАНИЕ: Используется временный токен! Настройте получение реального токена.")

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
