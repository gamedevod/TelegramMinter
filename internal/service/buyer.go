package service

import (
	"context"
	"fmt"
	"sync"
	"time"

	"stickersbot/internal/client"
	"stickersbot/internal/config"
	"stickersbot/internal/types"
)

// BuyerService сервис для покупки стикеров
type BuyerService struct {
	client     *client.HTTPClient
	config     *config.Config
	statistics *types.Statistics
	isRunning  bool
	cancel     context.CancelFunc
	mu         sync.RWMutex
	logChan    chan string
}

// NewBuyerService создает новый сервис покупки
func NewBuyerService(cfg *config.Config) *BuyerService {
	return &BuyerService{
		client:     client.New(),
		config:     cfg,
		statistics: &types.Statistics{},
		logChan:    make(chan string, 1000),
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
		return fmt.Errorf("неверная конфигурация: проверьте токен и количество потоков")
	}

	ctx, cancel := context.WithCancel(context.Background())
	bs.cancel = cancel
	bs.isRunning = true

	// Инициализируем статистику
	bs.statistics = &types.Statistics{
		StartTime: time.Now(),
	}

	bs.logChan <- "🚀 Запуск покупки стикеров..."
	bs.logChan <- fmt.Sprintf("📊 Потоков: %d", bs.config.Threads)
	bs.logChan <- fmt.Sprintf("🎯 Коллекция: %d, Персонаж: %d", bs.config.Collection, bs.config.Character)

	// Запускаем воркеры
	var wg sync.WaitGroup
	for i := 0; i < bs.config.Threads; i++ {
		wg.Add(1)
		go bs.worker(ctx, &wg, i+1)
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

// worker выполняет покупки в отдельном потоке
func (bs *BuyerService) worker(ctx context.Context, wg *sync.WaitGroup, workerID int) {
	defer wg.Done()

	for {
		select {
		case <-ctx.Done():
			bs.logChan <- fmt.Sprintf("🔄 Поток %d завершен", workerID)
			return
		default:
			bs.performBuy(workerID)
			time.Sleep(100 * time.Millisecond) // Небольшая задержка между запросами
		}
	}
}

// performBuy выполняет одну покупку
func (bs *BuyerService) performBuy(workerID int) {
	bs.mu.Lock()
	bs.statistics.TotalRequests++
	bs.mu.Unlock()

	resp, err := bs.client.BuyStickers(
		bs.config.AuthToken,
		bs.config.Collection,
		bs.config.Character,
		bs.config.Currency,
		bs.config.Count,
	)
	
	if err != nil {
		bs.mu.Lock()
		bs.statistics.FailedRequests++
		bs.mu.Unlock()
		
		bs.logChan <- fmt.Sprintf("❌ Поток %d: Ошибка запроса - %v", workerID, err)
		return
	}

	// Логируем полный ответ сервера
	bs.logChan <- fmt.Sprintf("📡 Поток %d: Статус %d", workerID, resp.StatusCode)
	bs.logChan <- fmt.Sprintf("📄 Поток %d: Ответ - %s", workerID, resp.Body)

	if resp.IsTokenError {
		bs.mu.Lock()
		bs.statistics.FailedRequests++
		bs.statistics.InvalidTokens++
		bs.mu.Unlock()

		bs.logChan <- fmt.Sprintf("🔑 Поток %d: Неверный токен авторизации!", workerID)
		// Останавливаем все потоки при неверном токене
		bs.Stop()
		return
	}

	if !resp.Success {
		bs.mu.Lock()
		bs.statistics.FailedRequests++
		bs.mu.Unlock()
		
		bs.logChan <- fmt.Sprintf("⚠️ Поток %d: Неуспешный запрос (статус %d)", workerID, resp.StatusCode)
	} else {
		bs.mu.Lock()
		bs.statistics.SuccessRequests++
		bs.mu.Unlock()
		
		bs.logChan <- fmt.Sprintf("✅ Поток %d: Успешный запрос!", workerID)
	}
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
			bs.logChan <- fmt.Sprintf("📈 Всего: %d | Успешно: %d | Ошибок: %d | InvalidTokens: %d | RPS: %.1f | Время: %s",
				stats.TotalRequests,
				stats.SuccessRequests,
				stats.FailedRequests,
				stats.InvalidTokens,
				stats.RequestsPerSec,
				stats.Duration.Truncate(time.Second),
			)
		}
	}
} 