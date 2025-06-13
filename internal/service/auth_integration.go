package service

import (
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	"stickersbot/internal/config"
	"stickersbot/internal/telegram"
)

// AuthIntegration интегрирует Telegram авторизацию в основной сервис
type AuthIntegration struct {
	config *config.Config
}

// NewAuthIntegration создает новый интеграционный сервис
func NewAuthIntegration(cfg *config.Config) *AuthIntegration {
	return &AuthIntegration{config: cfg}
}

// AuthorizeAccounts авторизует все аккаунты с номерами телефонов
func (ai *AuthIntegration) AuthorizeAccounts(ctx context.Context) error {
	for i, account := range ai.config.Accounts {
		// Проверяем, нужна ли Telegram авторизация
		if ai.shouldUseTelegramAuth(&account) {
			fmt.Printf("🔐 Авторизация Telegram для аккаунта: %s\n", account.Name)

			token, err := ai.authorizeAccount(ctx, &account)
			if err != nil {
				return fmt.Errorf("авторизация аккаунта %s: %w", account.Name, err)
			}

			// Обновляем токен в конфигурации
			ai.config.Accounts[i].AuthToken = token
			fmt.Printf("✅ Токен получен для аккаунта: %s\n", account.Name)
		}
	}

	return nil
}

// shouldUseTelegramAuth определяет, нужна ли Telegram авторизация
func (ai *AuthIntegration) shouldUseTelegramAuth(account *config.Account) bool {
	// Используем Telegram авторизацию если:
	// 1. Указан номер телефона
	// 2. Указаны API credentials
	// 3. Нет готового auth_token или он устарел
	return account.PhoneNumber != "" &&
		account.APIId != 0 &&
		account.APIHash != "" &&
		(account.AuthToken == "" || ai.isTokenExpired(account.AuthToken))
}

// isTokenExpired проверяет, истек ли токен (базовая проверка)
func (ai *AuthIntegration) isTokenExpired(token string) bool {
	// Простая проверка - если токен содержит timestamp, проверяем его
	if strings.Contains(token, "tg_token_") {
		// Можно добавить более сложную логику проверки
		return false // пока считаем, что токены не истекают
	}
	return false
}

// authorizeAccount авторизует конкретный аккаунт
func (ai *AuthIntegration) authorizeAccount(ctx context.Context, account *config.Account) (string, error) {
	// Определяем путь к файлу сессии
	sessionFile := account.SessionFile
	if sessionFile == "" {
		// Создаем имя файла сессии на основе номера телефона
		cleanPhone := strings.ReplaceAll(account.PhoneNumber, "+", "")
		sessionFile = filepath.Join("sessions", fmt.Sprintf("%s.session", cleanPhone))
	}

	// Создаем директорию для сессий если её нет
	sessionDir := filepath.Dir(sessionFile)
	if err := os.MkdirAll(sessionDir, 0755); err != nil {
		return "", fmt.Errorf("создание директории сессий %s: %w", sessionDir, err)
	}

	log.Printf("📁 Session файл будет создан/использован: %s", sessionFile)

	// Создаем сервис авторизации
	authService := telegram.NewAuthService(
		account.APIId,
		account.APIHash,
		account.PhoneNumber,
		sessionFile,
		account.BotUsername,
		account.WebAppURL,
		account.TokenAPIURL,
	)

	// Авторизуемся и получаем токен
	ctx, cancel := context.WithTimeout(ctx, 5*time.Minute) // таймаут на авторизацию
	defer cancel()

	token, err := authService.AuthorizeAndGetToken(ctx)
	if err != nil {
		return "", fmt.Errorf("ошибка авторизации: %w", err)
	}

	return token, nil
}

// ValidateAccounts проверяет валидность настроек Telegram авторизации
func (ai *AuthIntegration) ValidateAccounts() []error {
	var errors []error

	for _, account := range ai.config.Accounts {
		if ai.shouldUseTelegramAuth(&account) {
			if account.PhoneNumber == "" {
				errors = append(errors, fmt.Errorf("аккаунт %s: не указан номер телефона", account.Name))
			}

			if account.APIId == 0 {
				errors = append(errors, fmt.Errorf("аккаунт %s: не указан API ID", account.Name))
			}

			if account.APIHash == "" {
				errors = append(errors, fmt.Errorf("аккаунт %s: не указан API Hash", account.Name))
			}

			// Проверяем формат номера телефона
			if !strings.HasPrefix(account.PhoneNumber, "+") {
				errors = append(errors, fmt.Errorf("аккаунт %s: номер телефона должен начинаться с +", account.Name))
			}
		}
	}

	return errors
}
