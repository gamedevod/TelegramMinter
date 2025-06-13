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

	// Инициализируем статистику
	bs.statistics = &types.Statistics{
		StartTime: time.Now(),
	}

	bs.logChan <- "🚀 Запуск покупки стикеров..."
	bs.logChan <- fmt.Sprintf("📊 Аккаунтов: %d", len(bs.config.Accounts))

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

		// Запускаем потоки для этого аккаунта
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

	for {
		select {
		case <-ctx.Done():
			bs.logChan <- fmt.Sprintf("🔄 Поток %d (Аккаунт %d '%s') завершен", worker.workerID, accountNum, worker.account.Name)
			return
		default:
			// Проверяем активность аккаунта
			worker.mu.RLock()
			isActive := worker.isActive
			txCount := worker.transactionCount
			worker.mu.RUnlock()

			if !isActive {
				bs.logChan <- fmt.Sprintf("🛑 Поток %d (Аккаунт %d '%s') остановлен: достигнут лимит транзакций (%d/%d)",
					worker.workerID, accountNum, worker.account.Name, txCount, worker.account.MaxTransactions)
				return
			}

			bs.performAccountBuy(worker, accountNum)
			time.Sleep(100 * time.Millisecond) // Небольшая задержка между запросами
		}
	}
}

// performAccountBuy выполняет одну покупку для аккаунта
func (bs *BuyerService) performAccountBuy(worker *AccountWorker, accountNum int) {
	bs.mu.Lock()
	bs.statistics.TotalRequests++
	bs.mu.Unlock()

	// Проверяем, есть ли seed фраза для отправки транзакций
	if worker.account.SeedPhrase != "" {
		// Используем новый метод с отправкой TON транзакции
		resp, err := worker.client.BuyStickersAndPay(
			worker.account.AuthToken,
			worker.account.Collection,
			worker.account.Character,
			worker.account.Currency,
			worker.account.Count,
			worker.account.SeedPhrase,
			worker.testMode,
			worker.testAddr,
		)
		bs.handleAccountResponse(resp, err, worker, accountNum, true)
	} else {
		// Используем обычный метод без отправки транзакций
		resp, err := worker.client.BuyStickers(
			worker.account.AuthToken,
			worker.account.Collection,
			worker.account.Character,
			worker.account.Currency,
			worker.account.Count,
		)
		bs.handleAccountResponse(resp, err, worker, accountNum, false)
	}
}

// handleAccountResponse обрабатывает ответ от API для аккаунта
func (bs *BuyerService) handleAccountResponse(resp *client.BuyStickersResponse, err error, worker *AccountWorker, accountNum int, withTON bool) {
	if err != nil {
		bs.mu.Lock()
		bs.statistics.FailedRequests++
		bs.mu.Unlock()

		bs.logChan <- fmt.Sprintf("❌ Поток %d (Аккаунт %d '%s'): Ошибка - %v", worker.workerID, accountNum, worker.account.Name, err)
		return
	}

	// Логируем ответ сервера
	bs.logChan <- fmt.Sprintf("📡 Поток %d (Аккаунт %d '%s'): Статус %d", worker.workerID, accountNum, worker.account.Name, resp.StatusCode)
	bs.logChan <- fmt.Sprintf("📄 Поток %d (Аккаунт %d '%s'): Ответ - %s", worker.workerID, accountNum, worker.account.Name, resp.Body)

	if resp.IsTokenError {
		bs.mu.Lock()
		bs.statistics.FailedRequests++
		bs.statistics.InvalidTokens++
		bs.mu.Unlock()

		bs.logChan <- fmt.Sprintf("🔑 Поток %d (Аккаунт %d '%s'): Неверный токен авторизации!", worker.workerID, accountNum, worker.account.Name)
		return
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
		if withTON && resp.TransactionSent && resp.TransactionResult != nil {
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

		} else if withTON && resp.OrderID != "" {
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
