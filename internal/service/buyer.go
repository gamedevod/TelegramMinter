package service

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"sync"
	"time"

	"stickersbot/internal/client"
	"stickersbot/internal/config"
	"stickersbot/internal/monitor"
	"stickersbot/internal/types"
)

// AccountWorker структура для работы с отдельным аккаунтом
type AccountWorker struct {
	client           *client.HTTPClient
	account          config.Account
	testMode         bool
	testAddr         string
	workerID         int
	transactionCount int          // Счетчик успешных транзакций
	isActive         bool         // Флаг активности аккаунта
	mu               sync.RWMutex // Мьютекс для безопасного доступа к счетчикам
}

// BuyerService сервис для покупки стикеров
type BuyerService struct {
	client         *client.HTTPClient
	config         *config.Config
	statistics     *types.Statistics
	isRunning      bool
	cancel         context.CancelFunc
	mu             sync.RWMutex
	logChan        chan string
	transactionLog *os.File // Файл для логирования транзакций

	// Снайп мониторы
	snipeMonitors []*monitor.SnipeMonitor

	// Менеджер токенов
	tokenManager *TokenManager
}

// NewBuyerService создает новый сервис покупки
func NewBuyerService(cfg *config.Config) *BuyerService {
	// Создаем файл для логирования транзакций
	logFile, err := os.OpenFile("transactions.log", os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		fmt.Printf("⚠️ Не удалось создать файл логов транзакций: %v\n", err)
		logFile = nil
	}

	return &BuyerService{
		client:         client.New(),
		config:         cfg,
		statistics:     &types.Statistics{},
		logChan:        make(chan string, 1000),
		transactionLog: logFile,
		tokenManager:   NewTokenManager(cfg),
	}
}

// Start запускает процесс покупки стикеров
func (bs *BuyerService) Start() error {
	bs.mu.Lock()
	defer bs.mu.Unlock()

	if bs.isRunning {
		return fmt.Errorf("сервис уже запущен")
	}

	if !bs.config.IsValid() {
		return fmt.Errorf("неверная конфигурация: проверьте аккаунты")
	}

	ctx, cancel := context.WithCancel(context.Background())
	bs.cancel = cancel
	bs.isRunning = true

	// Создаем менеджер токенов
	bs.tokenManager = NewTokenManager(bs.config)

	// Инициализируем кеш токенов
	bs.tokenManager.InitializeTokens()

	// Запускаем превентивное обновление токенов каждые 30 минут
	go func() {
		ticker := time.NewTicker(30 * time.Minute)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				bs.tokenManager.PreventiveRefresh()
			case <-ctx.Done():
				return
			}
		}
	}()

	// Инициализируем статистику
	bs.statistics = &types.Statistics{
		StartTime: time.Now(),
	}

	bs.logChan <- "🚀 Запуск покупки стикеров..."
	bs.logChan <- fmt.Sprintf("📊 Аккаунтов: %d", len(bs.config.Accounts))

	// Инициализируем токены из конфигурации
	bs.logChan <- "🔍 Инициализация токенов авторизации..."

	// Подсчитываем общее количество потоков
	totalThreads := 0
	for _, account := range bs.config.Accounts {
		totalThreads += account.Threads
	}
	bs.logChan <- fmt.Sprintf("🔄 Общее количество потоков: %d", totalThreads)

	if bs.config.TestMode {
		bs.logChan <- fmt.Sprintf("🧪 ТЕСТОВЫЙ РЕЖИМ: платежи будут отправляться на %s", bs.config.TestAddress)
	} else {
		bs.logChan <- "⚠️ БОЕВОЙ РЕЖИМ: платежи будут отправляться на адреса из API"
	}

	// Запускаем воркеры для каждого аккаунта
	var wg sync.WaitGroup
	workerCounter := 0

	for accountIndex, account := range bs.config.Accounts {
		bs.logChan <- fmt.Sprintf("🎯 Аккаунт '%s': Коллекция: %d, Персонаж: %d, Валюта: %s, Количество: %d, Потоков: %d",
			account.Name, account.Collection, account.Character, account.Currency, account.Count, account.Threads)

		if account.SeedPhrase != "" {
			bs.logChan <- fmt.Sprintf("🔐 Аккаунт '%s': TON кошелек настроен", account.Name)
		} else {
			bs.logChan <- fmt.Sprintf("⚠️ Аккаунт '%s': TON кошелек НЕ настроен", account.Name)
		}

		// Проверяем, нужно ли запустить снайп монитор для этого аккаунта
		if account.SnipeMonitor != nil && account.SnipeMonitor.Enabled {
			bs.logChan <- fmt.Sprintf("🎯 Аккаунт '%s': Запуск снайп монитора", account.Name)

			// Создаем callback функцию для покупки
			purchaseCallback := bs.createPurchaseCallback(&account)

			// Создаем callback для получения токена
			tokenCallback := func(accountName string) (string, error) {
				return bs.tokenManager.GetValidToken(accountName)
			}

			// Создаем callback для обновления токена
			tokenRefreshCallback := func(accountName string, statusCode int) (string, error) {
				return bs.tokenManager.RefreshTokenOnError(accountName, statusCode)
			}

			// Создаем и запускаем снайп монитор
			snipeMonitor := monitor.NewSnipeMonitor(&account, client.New(), purchaseCallback, tokenCallback, tokenRefreshCallback)
			bs.snipeMonitors = append(bs.snipeMonitors, snipeMonitor)

			if err := snipeMonitor.Start(); err != nil {
				bs.logChan <- fmt.Sprintf("❌ Ошибка запуска снайп монитора для аккаунта '%s': %v", account.Name, err)
			}
		} else {
			// Запускаем обычные потоки для этого аккаунта
			for i := 0; i < account.Threads; i++ {
				wg.Add(1)
				workerCounter++

				accountWorker := &AccountWorker{
					client:           client.New(),
					account:          account,
					testMode:         bs.config.TestMode,
					testAddr:         bs.config.TestAddress,
					workerID:         workerCounter,
					transactionCount: 0,
					isActive:         true,
				}

				go bs.accountWorker(ctx, &wg, accountWorker, accountIndex+1)
			}
		}
	}

	// Запускаем горутину для обновления статистики
	go bs.updateStatistics(ctx)

	// Ждем завершения в отдельной горутине
	go func() {
		wg.Wait()
		bs.mu.Lock()
		bs.isRunning = false
		bs.mu.Unlock()
		bs.logChan <- "✅ Все потоки завершены"
	}()

	return nil
}

// accountWorker выполняет покупки для конкретного аккаунта
func (bs *BuyerService) accountWorker(ctx context.Context, wg *sync.WaitGroup, worker *AccountWorker, accountNum int) {
	defer wg.Done()

	bs.logChan <- fmt.Sprintf("🔄 Поток %d запущен для аккаунта %d '%s'", worker.workerID, accountNum, worker.account.Name)

	for {
		select {
		case <-ctx.Done():
			bs.logChan <- fmt.Sprintf("🛑 Поток %d остановлен", worker.workerID)
			return
		default:
			// Проверяем, активен ли аккаунт
			worker.mu.RLock()
			isActive := worker.isActive
			worker.mu.RUnlock()

			if !isActive {
				bs.logChan <- fmt.Sprintf("🛑 Поток %d неактивен (достигнут лимит транзакций)", worker.workerID)
				return
			}

			bs.performAccountBuy(worker, accountNum)
			time.Sleep(100 * time.Millisecond) // Небольшая задержка между запросами
		}
	}
}

// performAccountBuy выполняет покупку для конкретного аккаунта
func (bs *BuyerService) performAccountBuy(worker *AccountWorker, accountNum int) {
	// Получаем кешированный токен (без API проверки)
	bearerToken, err := bs.tokenManager.GetValidToken(worker.account.Name)
	if err != nil {
		bs.mu.Lock()
		bs.statistics.FailedRequests++
		bs.mu.Unlock()
		bs.logChan <- fmt.Sprintf("❌ Поток %d (Аккаунт %d '%s'): Ошибка получения токена: %v",
			worker.workerID, accountNum, worker.account.Name, err)
		return
	}

	// Выполняем запрос на покупку
	resp, err := bs.makeOrderRequest(worker.account, bearerToken)
	if err != nil {
		bs.mu.Lock()
		bs.statistics.FailedRequests++
		bs.mu.Unlock()
		bs.logChan <- fmt.Sprintf("❌ Поток %d (Аккаунт %d '%s'): Ошибка запроса: %v",
			worker.workerID, accountNum, worker.account.Name, err)
		return
	}

	// Проверяем статус ответа
	if resp.StatusCode == 401 || resp.StatusCode == 403 {
		// Токен истек, пытаемся обновить и повторить запрос
		bs.logChan <- fmt.Sprintf("🔄 Поток %d (Аккаунт %d '%s'): Токен истек (статус %d), обновляем...",
			worker.workerID, accountNum, worker.account.Name, resp.StatusCode)

		newToken, err := bs.tokenManager.RefreshTokenOnError(worker.account.Name, resp.StatusCode)
		if err != nil {
			bs.mu.Lock()
			bs.statistics.FailedRequests++
			bs.mu.Unlock()
			bs.logChan <- fmt.Sprintf("❌ Поток %d (Аккаунт %d '%s'): Ошибка обновления токена: %v",
				worker.workerID, accountNum, worker.account.Name, err)
			return
		}

		// Повторяем запрос с новым токеном
		resp2, err := bs.makeOrderRequest(worker.account, newToken)
		if err != nil {
			bs.mu.Lock()
			bs.statistics.FailedRequests++
			bs.mu.Unlock()
			bs.logChan <- fmt.Sprintf("❌ Поток %d (Аккаунт %d '%s'): Ошибка повторного запроса: %v",
				worker.workerID, accountNum, worker.account.Name, err)
			return
		}
		resp = resp2 // Используем новый ответ
	}

	// Логируем ответ сервера
	bs.logChan <- fmt.Sprintf("📡 Поток %d (Аккаунт %d '%s'): Статус %d", worker.workerID, accountNum, worker.account.Name, resp.StatusCode)
	bs.logChan <- fmt.Sprintf("📄 Поток %d (Аккаунт %d '%s'): Ответ - %s", worker.workerID, accountNum, worker.account.Name, resp.Body)

	if resp.IsTokenError {
		bs.mu.Lock()
		bs.statistics.FailedRequests++
		bs.statistics.InvalidTokens++
		bs.mu.Unlock()

		bs.logChan <- fmt.Sprintf("🔑 Поток %d (Аккаунт %d '%s'): Неверный токен авторизации! Попытка обновления...", worker.workerID, accountNum, worker.account.Name)

		// Пытаемся обновить токен
		newToken, err := bs.tokenManager.RefreshTokenOnError(worker.account.Name, resp.StatusCode)
		if err != nil {
			bs.logChan <- fmt.Sprintf("❌ Поток %d (Аккаунт %d '%s'): Не удалось обновить токен: %v", worker.workerID, accountNum, worker.account.Name, err)
			return
		}

		bs.logChan <- fmt.Sprintf("✅ Поток %d (Аккаунт %d '%s'): Токен успешно обновлен, повторяем запрос...", worker.workerID, accountNum, worker.account.Name)

		resp2, err := bs.makeOrderRequest(worker.account, newToken)
		if err != nil {
			bs.logChan <- fmt.Sprintf("❌ Поток %d (Аккаунт %d '%s'): Ошибка повторного запроса с новым токеном: %v", worker.workerID, accountNum, worker.account.Name, err)
			return
		}

		resp = resp2 // Используем новый ответ
		bs.logChan <- fmt.Sprintf("🔄 Поток %d (Аккаунт %d '%s'): Повторный запрос выполнен", worker.workerID, accountNum, worker.account.Name)
	}

	if !resp.Success {
		bs.mu.Lock()
		bs.statistics.FailedRequests++
		bs.mu.Unlock()

		bs.logChan <- fmt.Sprintf("⚠️ Поток %d (Аккаунт %d '%s'): Неуспешный запрос (статус %d)", worker.workerID, accountNum, worker.account.Name, resp.StatusCode)
	} else {
		// Успешный запрос
		bs.mu.Lock()
		bs.statistics.SuccessRequests++
		bs.mu.Unlock()

		// Обрабатываем транзакцию если она была отправлена
		if resp.TransactionSent && resp.TransactionResult != nil {
			// Обновляем глобальную статистику
			bs.mu.Lock()
			bs.statistics.SentTransactions++
			bs.mu.Unlock()

			// Обновляем счетчик транзакций для аккаунта
			worker.mu.Lock()
			worker.transactionCount++
			currentCount := worker.transactionCount

			// Проверяем, достиг ли аккаунт лимита транзакций
			if worker.account.MaxTransactions > 0 && currentCount >= worker.account.MaxTransactions {
				worker.isActive = false
				bs.logChan <- fmt.Sprintf("🛑 Аккаунт %d '%s' достиг лимита транзакций (%d/%d) и будет остановлен",
					accountNum, worker.account.Name, currentCount, worker.account.MaxTransactions)
			}
			worker.mu.Unlock()

			// Логируем информацию о транзакции
			txResult := resp.TransactionResult
			bs.logChan <- fmt.Sprintf("💰 Поток %d (Аккаунт %d '%s'): Транзакция отправлена!", worker.workerID, accountNum, worker.account.Name)
			bs.logChan <- fmt.Sprintf("   📤 С адреса: %s", txResult.FromAddress)
			bs.logChan <- fmt.Sprintf("   📥 На адрес: %s", txResult.ToAddress)
			bs.logChan <- fmt.Sprintf("   💰 Сумма: %.9f TON", float64(txResult.Amount)/1000000000)
			bs.logChan <- fmt.Sprintf("   🔗 Order ID: %s", resp.OrderID)
			bs.logChan <- fmt.Sprintf("   🆔 Transaction ID: %s", txResult.TransactionID)
			bs.logChan <- fmt.Sprintf("   📊 Транзакций аккаунта: %d/%d", currentCount, worker.account.MaxTransactions)

			// Записываем в файл логов
			txLog := &types.TransactionLog{
				Timestamp:     time.Now(),
				AccountName:   worker.account.Name,
				OrderID:       resp.OrderID,
				Amount:        txResult.Amount,
				Currency:      resp.Currency,
				FromAddress:   txResult.FromAddress,
				ToAddress:     txResult.ToAddress,
				TransactionID: txResult.TransactionID,
				TestMode:      worker.testMode,
			}
			bs.logTransaction(txLog)
		} else if resp.OrderID != "" {
			// Была попытка отправить транзакцию, но она не удалась
			bs.logChan <- fmt.Sprintf("✅ Поток %d (Аккаунт %d '%s'): Успешная покупка! OrderID: %s, но транзакция НЕ отправлена",
				worker.workerID, accountNum, worker.account.Name, resp.OrderID)
		} else {
			// Обычный успешный запрос без TON
			bs.logChan <- fmt.Sprintf("✅ Поток %d (Аккаунт %d '%s'): Успешный запрос!", worker.workerID, accountNum, worker.account.Name)
		}
	}
}

// Stop останавливает процесс покупки
func (bs *BuyerService) Stop() {
	bs.mu.Lock()
	defer bs.mu.Unlock()

	if !bs.isRunning {
		return
	}

	if bs.cancel != nil {
		bs.cancel()
	}

	// Останавливаем все снайп мониторы
	for _, monitor := range bs.snipeMonitors {
		monitor.Stop()
	}
	bs.snipeMonitors = nil

	// Закрываем файл логов транзакций
	if bs.transactionLog != nil {
		bs.transactionLog.Close()
		bs.transactionLog = nil
	}

	bs.logChan <- "🛑 Остановка покупки стикеров..."
}

// IsRunning возвращает статус работы сервиса
func (bs *BuyerService) IsRunning() bool {
	bs.mu.RLock()
	defer bs.mu.RUnlock()
	return bs.isRunning
}

// GetStatistics возвращает текущую статистику
func (bs *BuyerService) GetStatistics() *types.Statistics {
	bs.mu.RLock()
	defer bs.mu.RUnlock()

	// Создаем копию статистики
	stats := *bs.statistics
	if bs.isRunning {
		stats.Duration = time.Since(stats.StartTime)
		if stats.Duration.Seconds() > 0 {
			stats.RequestsPerSec = float64(stats.TotalRequests) / stats.Duration.Seconds()
		}
	}
	return &stats
}

// GetLogChannel возвращает канал для получения логов
func (bs *BuyerService) GetLogChannel() <-chan string {
	return bs.logChan
}

// updateStatistics обновляет статистику каждую секунду
func (bs *BuyerService) updateStatistics(ctx context.Context) {
	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			stats := bs.GetStatistics()
			bs.logChan <- fmt.Sprintf("📈 Всего: %d | Успешно: %d | Ошибок: %d | InvalidTokens: %d | TON отправлено: %d | RPS: %.1f | Время: %s",
				stats.TotalRequests,
				stats.SuccessRequests,
				stats.FailedRequests,
				stats.InvalidTokens,
				stats.SentTransactions,
				stats.RequestsPerSec,
				stats.Duration.Truncate(time.Second),
			)
		}
	}
}

// logTransaction записывает информацию о транзакции в файл
func (bs *BuyerService) logTransaction(txLog *types.TransactionLog) {
	if bs.transactionLog == nil {
		return
	}

	// Преобразуем в JSON
	data, err := json.Marshal(txLog)
	if err != nil {
		bs.logChan <- fmt.Sprintf("❌ Ошибка записи лога транзакции: %v", err)
		return
	}

	// Записываем в файл
	_, err = bs.transactionLog.WriteString(string(data) + "\n")
	if err != nil {
		bs.logChan <- fmt.Sprintf("❌ Ошибка записи в файл лога: %v", err)
		return
	}

	// Сразу сохраняем на диск
	bs.transactionLog.Sync()
}

// createPurchaseCallback создает callback функцию для покупки стикеров
func (bs *BuyerService) createPurchaseCallback(account *config.Account) monitor.PurchaseCallback {
	return func(request monitor.PurchaseRequest) error {
		bs.logChan <- fmt.Sprintf("🚀 Снайп покупка: %s (Коллекция: %d, Персонаж: %d, Цена: %d)",
			request.Name, request.CollectionID, request.CharacterID, request.Price)

		return bs.performSnipePurchase(account.Name, request.CollectionID, request.CharacterID)
	}
}

// performSnipePurchase выполняет покупку через snipe monitor
func (bs *BuyerService) performSnipePurchase(accountName string, collectionID int, characterID int) error {
	// Получаем кешированный токен (без API проверки)
	bearerToken, err := bs.tokenManager.GetValidToken(accountName)
	if err != nil {
		return fmt.Errorf("ошибка получения токена: %v", err)
	}

	// Находим аккаунт в конфигурации
	var account *config.Account
	for _, acc := range bs.config.Accounts {
		if acc.Name == accountName {
			account = &acc
			break
		}
	}
	if account == nil {
		return fmt.Errorf("аккаунт %s не найден", accountName)
	}

	// Выполняем запрос на покупку
	resp, err := bs.makeSnipeOrderRequest(*account, bearerToken, collectionID, characterID)
	if err != nil {
		return fmt.Errorf("ошибка запроса: %v", err)
	}

	// Проверяем статус ответа
	if resp.StatusCode == 401 || resp.StatusCode == 403 {
		// Токен истек, пытаемся обновить и повторить запрос
		bs.logChan <- fmt.Sprintf("🔄 [%s] Токен истек при snipe (статус %d), обновляем...", accountName, resp.StatusCode)

		newToken, err := bs.tokenManager.RefreshTokenOnError(accountName, resp.StatusCode)
		if err != nil {
			return fmt.Errorf("ошибка обновления токена: %v", err)
		}

		// Повторяем запрос с новым токеном
		resp2, err := bs.makeSnipeOrderRequest(*account, newToken, collectionID, characterID)
		if err != nil {
			return fmt.Errorf("ошибка повторного запроса: %v", err)
		}
		resp = resp2 // Используем новый ответ
	}

	// Логируем ответ сервера
	bs.logChan <- fmt.Sprintf("📡 Снайп '%s': Статус %d", account.Name, resp.StatusCode)
	bs.logChan <- fmt.Sprintf("📄 Снайп '%s': Ответ - %s", account.Name, resp.Body)

	if resp.IsTokenError {
		bs.mu.Lock()
		bs.statistics.FailedRequests++
		bs.statistics.InvalidTokens++
		bs.mu.Unlock()

		bs.logChan <- fmt.Sprintf("🔑 Снайп '%s': Неверный токен авторизации! Попытка обновления...", account.Name)

		// Пытаемся обновить токен
		newToken, err := bs.tokenManager.RefreshTokenOnError(account.Name, resp.StatusCode)
		if err != nil {
			bs.logChan <- fmt.Sprintf("❌ Снайп '%s': Не удалось обновить токен: %v", account.Name, err)
			return nil
		}

		bs.logChan <- fmt.Sprintf("✅ Снайп '%s': Токен успешно обновлен, повторяем запрос...", account.Name)

		// Повторяем запрос с новым токеном
		resp2, err := bs.makeSnipeOrderRequest(*account, newToken, collectionID, characterID)
		if err != nil {
			bs.logChan <- fmt.Sprintf("❌ Снайп '%s': Ошибка повторного запроса с новым токеном: %v", account.Name, err)
			return nil
		}

		resp = resp2 // Используем новый ответ
		bs.logChan <- fmt.Sprintf("🔄 Снайп '%s': Повторный запрос выполнен", account.Name)
	}

	if !resp.Success {
		bs.mu.Lock()
		bs.statistics.FailedRequests++
		bs.mu.Unlock()

		bs.logChan <- fmt.Sprintf("⚠️ Снайп '%s': Неуспешный запрос (статус %d)", account.Name, resp.StatusCode)
		return nil
	}

	// Успешный запрос
	bs.mu.Lock()
	bs.statistics.SuccessRequests++
	bs.mu.Unlock()

	// Обрабатываем транзакцию если она была отправлена
	if resp.TransactionSent && resp.TransactionResult != nil {
		// Обновляем глобальную статистику
		bs.mu.Lock()
		bs.statistics.SentTransactions++
		bs.mu.Unlock()

		// Логируем информацию о транзакции
		txResult := resp.TransactionResult
		bs.logChan <- fmt.Sprintf("💰 Снайп '%s': Транзакция отправлена!", account.Name)
		bs.logChan <- fmt.Sprintf("   📤 С адреса: %s", txResult.FromAddress)
		bs.logChan <- fmt.Sprintf("   📥 На адрес: %s", txResult.ToAddress)
		bs.logChan <- fmt.Sprintf("   💰 Сумма: %.9f TON", float64(txResult.Amount)/1000000000)
		bs.logChan <- fmt.Sprintf("   🔗 Order ID: %s", resp.OrderID)
		bs.logChan <- fmt.Sprintf("   🆔 Transaction ID: %s", txResult.TransactionID)

		// Записываем в файл логов
		txLog := &types.TransactionLog{
			Timestamp:     time.Now(),
			AccountName:   account.Name,
			OrderID:       resp.OrderID,
			Amount:        txResult.Amount,
			Currency:      resp.Currency,
			FromAddress:   txResult.FromAddress,
			ToAddress:     txResult.ToAddress,
			TransactionID: txResult.TransactionID,
			TestMode:      bs.config.TestMode,
		}
		bs.logTransaction(txLog)
	}

	return nil
}

// makeOrderRequest выполняет HTTP запрос на покупку
func (bs *BuyerService) makeOrderRequest(account config.Account, bearerToken string) (*client.BuyStickersResponse, error) {
	bs.mu.Lock()
	bs.statistics.TotalRequests++
	bs.mu.Unlock()

	httpClient := client.New()

	// Проверяем, есть ли seed фраза для отправки транзакций
	if account.SeedPhrase != "" {
		// Используем новый метод с отправкой TON транзакции
		return httpClient.BuyStickersAndPay(
			bearerToken,
			account.Collection,
			account.Character,
			account.Currency,
			account.Count,
			account.SeedPhrase,
			bs.config.TestMode,
			bs.config.TestAddress,
		)
	} else {
		// Используем обычный метод без отправки транзакций
		return httpClient.BuyStickers(
			bearerToken,
			account.Collection,
			account.Character,
			account.Currency,
			account.Count,
		)
	}
}

// makeSnipeOrderRequest выполняет HTTP запрос на покупку через snipe monitor
func (bs *BuyerService) makeSnipeOrderRequest(account config.Account, bearerToken string, collectionID int, characterID int) (*client.BuyStickersResponse, error) {
	bs.mu.Lock()
	bs.statistics.TotalRequests++
	bs.mu.Unlock()

	httpClient := client.New()

	// Проверяем, есть ли seed фраза для отправки транзакций
	if account.SeedPhrase != "" {
		// Используем новый метод с отправкой TON транзакции
		return httpClient.BuyStickersAndPay(
			bearerToken,
			collectionID,
			characterID,
			account.Currency,
			account.Count,
			account.SeedPhrase,
			bs.config.TestMode,
			bs.config.TestAddress,
		)
	} else {
		// Используем обычный метод без отправки транзакций
		return httpClient.BuyStickers(
			bearerToken,
			collectionID,
			characterID,
			account.Currency,
			account.Count,
		)
	}
}
