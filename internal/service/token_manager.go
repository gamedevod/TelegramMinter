package service

import (
	"context"
	"fmt"
	"log"
	"strings"
	"sync"
	"time"

	"stickersbot/internal/client"
	"stickersbot/internal/config"
	"stickersbot/internal/telegram"
)

// TokenInfo информация о токене с кешированием
type TokenInfo struct {
	Token     string    `json:"token"`
	ExpiresAt time.Time `json:"expires_at"`
	IsValid   bool      `json:"is_valid"`
	LastCheck time.Time `json:"last_check"`
}

// TokenManager управляет Bearer токенами аккаунтов с кешированием
type TokenManager struct {
	config      *config.Config
	httpClient  *client.HTTPClient
	tokens      map[string]*TokenInfo // ключ - имя аккаунта
	mutex       sync.RWMutex
	authService *AuthIntegration

	// Настройки кеширования
	tokenTTL      time.Duration // Время жизни токена (по умолчанию 40 минут)
	checkCooldown time.Duration // Минимальный интервал между проверками (по умолчанию 1 минута)
}

// NewTokenManager создает новый менеджер токенов
func NewTokenManager(cfg *config.Config) *TokenManager {
	return &TokenManager{
		config:        cfg,
		httpClient:    client.New(),
		tokens:        make(map[string]*TokenInfo),
		authService:   NewAuthIntegration(cfg),
		tokenTTL:      40 * time.Minute, // Токены живут ~45 минут, обновляем за 5 минут до истечения
		checkCooldown: 1 * time.Minute,  // Не проверяем чаще раза в минуту
	}
}

// GetCachedToken возвращает кешированный токен без проверки API
func (tm *TokenManager) GetCachedToken(accountName string) (string, error) {
	tm.mutex.RLock()
	defer tm.mutex.RUnlock()

	// Находим аккаунт в конфигурации
	var account *config.Account
	for _, acc := range tm.config.Accounts {
		if acc.Name == accountName {
			account = &acc
			break
		}
	}

	if account == nil {
		return "", fmt.Errorf("аккаунт %s не найден", accountName)
	}

	// Проверяем кешированный токен
	if tokenInfo, exists := tm.tokens[accountName]; exists {
		// Если токен еще не истек по нашему TTL, возвращаем его
		if time.Now().Before(tokenInfo.ExpiresAt) {
			return tokenInfo.Token, nil
		}
	}

	// Если кеша нет или токен истек, возвращаем токен из конфигурации
	if account.AuthToken != "" {
		// Обновляем кеш с текущим токеном
		tm.tokens[accountName] = &TokenInfo{
			Token:     account.AuthToken,
			ExpiresAt: time.Now().Add(tm.tokenTTL),
			IsValid:   true,
			LastCheck: time.Now(),
		}
		return account.AuthToken, nil
	}

	return "", fmt.Errorf("токен для аккаунта %s отсутствует", accountName)
}

// RefreshTokenOnError обновляет токен только при получении ошибки авторизации
func (tm *TokenManager) RefreshTokenOnError(accountName string, statusCode int) (string, error) {
	tm.mutex.Lock()
	defer tm.mutex.Unlock()

	log.Printf("🔄 Обновление токена для %s из-за ошибки %d", accountName, statusCode)

	// Проверяем cooldown - не обновляем слишком часто, НО игнорируем cooldown для критических ошибок токена
	isTokenError := statusCode == 401 || statusCode == 403 || statusCode == 200 // 200 может содержать JSON ошибку токена
	if tokenInfo, exists := tm.tokens[accountName]; exists && !isTokenError {
		if time.Since(tokenInfo.LastCheck) < tm.checkCooldown {
			log.Printf("⏳ Слишком частое обновление токена для %s, используем кешированный", accountName)
			return tokenInfo.Token, nil
		}
	}

	// Для ошибок токена всегда пытаемся обновить
	if isTokenError {
		log.Printf("🔑 Критическая ошибка токена для %s (статус %d), принудительное обновление", accountName, statusCode)
	}

	// Находим аккаунт в конфигурации
	var account *config.Account
	var accountIndex int
	for i, acc := range tm.config.Accounts {
		if acc.Name == accountName {
			account = &acc
			accountIndex = i
			break
		}
	}

	if account == nil {
		return "", fmt.Errorf("аккаунт %s не найден", accountName)
	}

	// Обновляем токен через Telegram авторизацию
	log.Printf("🔄 Запуск Telegram авторизации для %s...", accountName)
	newToken, err := tm.refreshTokenViaTelegram(account)
	if err != nil {
		log.Printf("❌ Ошибка обновления токена для %s: %v", accountName, err)
		// Возвращаем старый токен если обновление не удалось
		if account.AuthToken != "" {
			log.Printf("🔄 Используем старый токен для %s", accountName)
			return account.AuthToken, nil
		}
		return "", fmt.Errorf("ошибка обновления токена для %s: %v", accountName, err)
	}

	tokenPreview := newToken
	if len(tokenPreview) > 20 {
		tokenPreview = tokenPreview[:20] + "..."
	}
	log.Printf("✅ Получен новый токен для %s: %s", accountName, tokenPreview)

	// Проверяем, отличается ли новый токен от старого
	if account.AuthToken == newToken {
		log.Printf("⚠️ Новый токен для %s идентичен старому! Возможна проблема с авторизацией", accountName)
	}

	// Проверяем, не является ли токен временным/невалидным (только для явно временных токенов)
	if strings.Contains(newToken, "INVALID_TEMP_TOKEN") {
		log.Printf("❌ Получен временный/невалидный токен для %s: %s", accountName, tokenPreview)
		log.Printf("❌ Этот токен НЕ БУДЕТ работать с API!")
		return "", fmt.Errorf("получен невалидный временный токен для %s", accountName)
	}

	// Сохраняем новый токен в конфигурацию
	tm.config.Accounts[accountIndex].AuthToken = newToken

	// Сохраняем конфигурацию в фоне (не блокируем основной поток)
	go func() {
		if err := tm.config.Save("config.json"); err != nil {
			log.Printf("⚠️ Не удалось сохранить конфигурацию: %v", err)
		}
	}()

	// Обновляем кеш
	tm.tokens[accountName] = &TokenInfo{
		Token:     newToken,
		ExpiresAt: time.Now().Add(tm.tokenTTL),
		IsValid:   true,
		LastCheck: time.Now(),
	}

	log.Printf("✅ Токен для аккаунта %s успешно обновлен", accountName)
	return newToken, nil
}

// refreshTokenViaTelegram обновляет токен через Telegram авторизацию
func (tm *TokenManager) refreshTokenViaTelegram(account *config.Account) (string, error) {
	if account.PhoneNumber == "" {
		return "", fmt.Errorf("номер телефона не указан для аккаунта %s", account.Name)
	}

	// Определяем путь к файлу сессии
	sessionFile := account.SessionFile
	if sessionFile == "" {
		cleanPhone := strings.ReplaceAll(account.PhoneNumber, "+", "")
		sessionFile = fmt.Sprintf("sessions/%s.session", cleanPhone)
	}

	// Создаем сервис авторизации
	authService := telegram.NewAuthService(
		tm.config.APIId,
		tm.config.APIHash,
		account.PhoneNumber,
		sessionFile,
		tm.config.BotUsername,
		tm.config.WebAppURL,
		tm.config.TokenAPIURL,
	)

	// Выполняем авторизацию с таймаутом
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	bearerToken, err := authService.AuthorizeAndGetToken(ctx)
	if err != nil {
		return "", fmt.Errorf("ошибка Telegram авторизации: %v", err)
	}

	return bearerToken, nil
}

// PreventiveRefresh превентивно обновляет токены которые скоро истекут
func (tm *TokenManager) PreventiveRefresh() {
	tm.mutex.Lock()
	defer tm.mutex.Unlock()

	log.Printf("🔄 Превентивное обновление токенов...")

	for accountName, tokenInfo := range tm.tokens {
		// Обновляем токены которые истекут в ближайшие 5 минут
		if time.Until(tokenInfo.ExpiresAt) < 5*time.Minute {
			log.Printf("⏰ Токен для %s скоро истечет, обновляем превентивно", accountName)

			// Запускаем обновление в отдельной горутине чтобы не блокировать
			go func(name string) {
				_, err := tm.RefreshTokenOnError(name, 401) // Принудительное обновление
				if err != nil {
					log.Printf("❌ Ошибка превентивного обновления токена для %s: %v", name, err)
				}
			}(accountName)
		}
	}
}

// GetValidToken возвращает действительный токен (основной метод для использования)
func (tm *TokenManager) GetValidToken(accountName string) (string, error) {
	return tm.GetCachedToken(accountName)
}

// InitializeTokens инициализирует кеш токенов из конфигурации
func (tm *TokenManager) InitializeTokens() {
	tm.mutex.Lock()
	defer tm.mutex.Unlock()

	log.Printf("🔧 Инициализация кеша токенов...")

	for _, account := range tm.config.Accounts {
		if account.AuthToken != "" {
			tm.tokens[account.Name] = &TokenInfo{
				Token:     account.AuthToken,
				ExpiresAt: time.Now().Add(tm.tokenTTL),
				IsValid:   true,
				LastCheck: time.Now(),
			}
			log.Printf("📋 Токен для %s добавлен в кеш", account.Name)
		}
	}
}

// RefreshTokenOnJSONError обновляет токен при получении JSON ошибки токена
func (tm *TokenManager) RefreshTokenOnJSONError(accountName string) (string, error) {
	log.Printf("🔑 Обновление токена для %s из-за JSON ошибки токена", accountName)
	return tm.RefreshTokenOnError(accountName, 200) // Используем статус 200 для JSON ошибок
}

// ForceRefreshToken принудительно обновляет токен (игнорируя кеш и cooldown)
func (tm *TokenManager) ForceRefreshToken(accountName string) (string, error) {
	tm.mutex.Lock()
	defer tm.mutex.Unlock()

	log.Printf("🔄 Принудительное обновление токена для %s", accountName)

	// Находим аккаунт в конфигурации
	var account *config.Account
	var accountIndex int
	for i, acc := range tm.config.Accounts {
		if acc.Name == accountName {
			account = &acc
			accountIndex = i
			break
		}
	}

	if account == nil {
		return "", fmt.Errorf("аккаунт %s не найден", accountName)
	}

	// Обновляем токен через Telegram авторизацию
	newToken, err := tm.refreshTokenViaTelegram(account)
	if err != nil {
		log.Printf("❌ Ошибка принудительного обновления токена для %s: %v", accountName, err)
		return "", fmt.Errorf("ошибка обновления токена для %s: %v", accountName, err)
	}

	// Сохраняем новый токен в конфигурацию
	tm.config.Accounts[accountIndex].AuthToken = newToken

	// Сохраняем конфигурацию
	if err := tm.config.Save("config.json"); err != nil {
		log.Printf("⚠️ Не удалось сохранить конфигурацию: %v", err)
	}

	// Обновляем кеш
	tm.tokens[accountName] = &TokenInfo{
		Token:     newToken,
		ExpiresAt: time.Now().Add(tm.tokenTTL),
		IsValid:   true,
		LastCheck: time.Now(),
	}

	log.Printf("✅ Токен для аккаунта %s принудительно обновлен", accountName)
	return newToken, nil
}

// InvalidateTokenCache очищает кеш токена для аккаунта
func (tm *TokenManager) InvalidateTokenCache(accountName string) {
	tm.mutex.Lock()
	defer tm.mutex.Unlock()

	delete(tm.tokens, accountName)
	log.Printf("🗑️ Кеш токена для %s очищен", accountName)
}

// ReloadTokenFromConfig перезагружает токен из конфигурации
func (tm *TokenManager) ReloadTokenFromConfig(accountName string) error {
	tm.mutex.Lock()
	defer tm.mutex.Unlock()

	// Находим аккаунт в конфигурации
	var account *config.Account
	for _, acc := range tm.config.Accounts {
		if acc.Name == accountName {
			account = &acc
			break
		}
	}

	if account == nil {
		return fmt.Errorf("аккаунт %s не найден", accountName)
	}

	if account.AuthToken == "" {
		return fmt.Errorf("токен для аккаунта %s отсутствует в конфигурации", accountName)
	}

	// Обновляем кеш с токеном из конфигурации
	tm.tokens[accountName] = &TokenInfo{
		Token:     account.AuthToken,
		ExpiresAt: time.Now().Add(tm.tokenTTL),
		IsValid:   true,
		LastCheck: time.Now(),
	}

	log.Printf("🔄 Токен для %s перезагружен из конфигурации", accountName)
	return nil
}
