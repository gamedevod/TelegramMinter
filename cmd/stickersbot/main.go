package main

import (
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"stickersbot/internal/config"
	"stickersbot/internal/service"
)

func main() {
	// Ищем конфигурацию
	cfgPath := findConfigPath()
	cfg, err := config.Load(cfgPath)
	if err != nil {
		fmt.Printf("❌ Ошибка загрузки конфигурации (%s): %v\n", cfgPath, err)
		os.Exit(1)
	}
	fmt.Printf("📋 Загружена конфигурация: %s\n", cfgPath)

	// Проверяем конфигурацию
	if !cfg.IsValid() {
		fmt.Println("❌ Конфигурация недействительна. Проверьте аккаунты и их настройки.")
		os.Exit(1)
	}

	// Создаём сервис покупки
	buyerService := service.NewBuyerService(cfg)

	// Запускаем сервис
	if err := buyerService.Start(); err != nil {
		fmt.Printf("❌ Ошибка запуска сервиса: %v\n", err)
		os.Exit(1)
	}

	fmt.Println("🚀 Покупка стикеров запущена! Нажмите Ctrl+C для остановки.")

	// Горутина для трансляции логов в stdout
	go func() {
		for log := range buyerService.GetLogChannel() {
			fmt.Println(log)
		}
	}()

	// Периодический вывод статистики каждые 5 секунд
	go func() {
		ticker := time.NewTicker(5 * time.Second)
		defer ticker.Stop()
		for range ticker.C {
			stats := buyerService.GetStatistics()
			fmt.Printf("📈 Всего: %d | Успешно: %d | Ошибок: %d | TON отправлено: %d | RPS: %.1f | Время: %s\n",
				stats.TotalRequests,
				stats.SuccessRequests,
				stats.FailedRequests,
				stats.SentTransactions,
				stats.RequestsPerSec,
				stats.Duration.Truncate(time.Second),
			)
		}
	}()

	// Перехватываем Ctrl+C / SIGTERM
	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, os.Interrupt, syscall.SIGTERM)
	<-sigs // Блокируемся до сигнала

	fmt.Println("\n🛑 Остановка...")
	buyerService.Stop()

	// Даём воркерам корректно завершиться
	time.Sleep(2 * time.Second)

	stats := buyerService.GetStatistics()
	fmt.Printf("✅ Завершено. Всего запросов: %d, Успешно: %d, Ошибок: %d, TON отправлено: %d.\n",
		stats.TotalRequests, stats.SuccessRequests, stats.FailedRequests, stats.SentTransactions)
}

// findConfigPath возвращает путь к конфигурационному файлу
func findConfigPath() string {
	return "./config.json"
}
