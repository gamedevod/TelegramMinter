package service

import (
	"context"
	"fmt"
	"log"

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

// AuthorizeAccounts выполняет авторизацию для всех аккаунтов, которым это требуется
func (ai *AuthIntegration) AuthorizeAccounts(ctx context.Context) error {
	for i, account := range ai.config.Accounts {
		if ai.needsTelegramAuth(account) {
			log.Printf("🔐 Авторизация Telegram для аккаунта: %s", account.Name)

			// Создаем сервис авторизации с общими параметрами
			authService := telegram.NewAuthService(
				ai.config.APIId,
				ai.config.APIHash,
				account.PhoneNumber,
				account.SessionFile,
				ai.config.BotUsername,
				ai.config.WebAppURL,
				ai.config.TokenAPIURL,
			)

			// Выполняем авторизацию
			bearerToken, err := authService.AuthorizeAndGetToken(ctx)
			if err != nil {
				return fmt.Errorf("ошибка авторизации аккаунта %s: %w", account.Name, err)
			}

			// Сохраняем полученный токен
			ai.config.Accounts[i].AuthToken = bearerToken
			log.Printf("✅ Авторизация завершена для аккаунта: %s", account.Name)
		} else if account.AuthToken != "" {
			log.Printf("✅ Аккаунт %s уже имеет Bearer токен", account.Name)
		} else {
			log.Printf("⚠️  Аккаунт %s не настроен для Telegram авторизации", account.Name)
		}
	}

	return nil
}

// ValidateAccounts проверяет корректность настроек Telegram авторизации
func (ai *AuthIntegration) ValidateAccounts() []error {
	var errors []error

	for _, account := range ai.config.Accounts {
		if ai.needsTelegramAuth(account) {
			if ai.config.APIId == 0 {
				errors = append(errors, fmt.Errorf("аккаунт %s: api_id не указан в общих настройках", account.Name))
			}

			if ai.config.APIHash == "" {
				errors = append(errors, fmt.Errorf("аккаунт %s: api_hash не указан в общих настройках", account.Name))
			}

			if account.PhoneNumber == "" {
				errors = append(errors, fmt.Errorf("аккаунт %s: phone_number не указан", account.Name))
			}
		}
	}

	return errors
}

// hasTelegramAuth проверяет, настроена ли Telegram авторизация для аккаунта
func (ai *AuthIntegration) hasTelegramAuth(account config.Account) bool {
	return account.PhoneNumber != "" &&
		ai.config.APIId != 0 &&
		ai.config.APIHash != ""
}

// needsTelegramAuth проверяет, нужна ли Telegram авторизация для аккаунта
func (ai *AuthIntegration) needsTelegramAuth(account config.Account) bool {
	return account.AuthToken == "" && ai.hasTelegramAuth(account)
}
