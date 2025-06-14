package monitor

import (
	"context"
	"fmt"
	"log"
	"strings"
	"sync"
	"time"

	"stickersbot/internal/client"
	"stickersbot/internal/config"
)

// PurchaseRequest структура запроса на покупку
type PurchaseRequest struct {
	CollectionID int
	CharacterID  int
	Price        int
	Supply       int
	Name         string
}

// PurchaseCallback функция обратного вызова для покупки
type PurchaseCallback func(request PurchaseRequest) error

// TokenCallback функция обратного вызова для получения валидного токена
type TokenCallback func(accountName string) (string, error)

// TokenRefreshCallback функция обратного вызова для обновления токена при ошибке
type TokenRefreshCallback func(accountName string, statusCode int) (string, error)

// SnipeMonitor структура снайп монитора
type SnipeMonitor struct {
	config               *config.Account
	apiClient            *APIClient
	httpClient           *client.HTTPClient
	purchaseCallback     PurchaseCallback
	tokenCallback        TokenCallback
	tokenRefreshCallback TokenRefreshCallback

	// Состояние
	knownCollections map[int]bool    // ID известных коллекций
	knownCharacters  map[string]bool // "collectionID:characterID" известных персонажей
	mutex            sync.RWMutex

	// Управление жизненным циклом
	ctx    context.Context
	cancel context.CancelFunc

	// Логирование
	logPrefix        string
	collectionLogger *CollectionLogger
}

// NewSnipeMonitor создает новый снайп монитор
func NewSnipeMonitor(account *config.Account, httpClient *client.HTTPClient, purchaseCallback PurchaseCallback, tokenCallback TokenCallback, tokenRefreshCallback TokenRefreshCallback) *SnipeMonitor {
	ctx, cancel := context.WithCancel(context.Background())

	// Создаем имя файла для логов коллекций
	logFilename := fmt.Sprintf("found_collections_%s.json", strings.ReplaceAll(account.Name, " ", "_"))

	return &SnipeMonitor{
		config:               account,
		apiClient:            NewAPIClient(httpClient),
		httpClient:           httpClient,
		purchaseCallback:     purchaseCallback,
		tokenCallback:        tokenCallback,
		tokenRefreshCallback: tokenRefreshCallback,
		knownCollections:     make(map[int]bool),
		knownCharacters:      make(map[string]bool),
		ctx:                  ctx,
		cancel:               cancel,
		logPrefix:            fmt.Sprintf("[SNIPE:%s]", account.Name),
		collectionLogger:     NewCollectionLogger(logFilename),
	}
}

// Start запускает снайп монитор
func (s *SnipeMonitor) Start() error {
	if s.config.SnipeMonitor == nil || !s.config.SnipeMonitor.Enabled {
		return fmt.Errorf("снайп\ монитор\ не\ включен")
	}

	if s.config.AuthToken == "" {
		return fmt.Errorf("отсутствует\ токен\ авторизации")
	}

	s.log("🎯 Снайп монитор запущен")
	s.log("📊 Настройки:")
	if s.config.SnipeMonitor.SupplyRange != nil {
		s.log("   Supply: %d - %d", s.config.SnipeMonitor.SupplyRange.Min, s.config.SnipeMonitor.SupplyRange.Max)
	}
	if s.config.SnipeMonitor.PriceRange != nil {
		s.log("   Price: %d - %d нанотон", s.config.SnipeMonitor.PriceRange.Min, s.config.SnipeMonitor.PriceRange.Max)
	}
	if len(s.config.SnipeMonitor.WordFilter) > 0 {
		s.log("   Фильтр слов: %v", s.config.SnipeMonitor.WordFilter)
	}

	// Инициализируем состояние - получаем текущие коллекции
	if err := s.initializeState(); err != nil {
		s.log("⚠️ Ошибка инициализации состояния: %v", err)
	}

	// Запускаем основной цикл мониторинга
	go s.monitorLoop()

	return nil
}

// Stop останавливает снайп монитор
func (s *SnipeMonitor) Stop() {
	s.log("🛑 Остановка снайп монитора")
	s.cancel()
}

// initializeState инициализирует состояние монитора
func (s *SnipeMonitor) initializeState() error {
	// Получаем валидный токен
	token, err := s.tokenCallback(s.config.Name)
	if err != nil {
		return fmt.Errorf("ошибка получения токена: %v", err)
	}

	collections, err := s.apiClient.GetCollections(token)
	if err != nil {
		// Проверяем, является ли это ошибкой токена
		if tokenErr, ok := err.(*TokenError); ok {
			s.log("🔑 Ошибка токена при инициализации: %v", tokenErr)
			// Пытаемся обновить токен
			newToken, refreshErr := s.tokenRefreshCallback(s.config.Name, tokenErr.StatusCode)
			if refreshErr != nil {
				return fmt.Errorf("ошибка обновления токена: %v", refreshErr)
			}
			token = newToken // Обновляем токен для дальнейшего использования
			// Повторяем запрос с новым токеном
			collections, err = s.apiClient.GetCollections(newToken)
			if err != nil {
				return fmt.Errorf("ошибка получения коллекций после обновления токена: %v", err)
			}
		} else {
			return fmt.Errorf("ошибка получения коллекций: %v", err)
		}
	}

	s.mutex.Lock()
	defer s.mutex.Unlock()

	// Запоминаем все существующие коллекции
	for _, collection := range collections.Data {
		s.knownCollections[collection.ID] = true

		// Получаем детали коллекции для запоминания персонажей
		details, err := s.apiClient.GetCollectionDetails(token, collection.ID)
		if err != nil {
			s.log("⚠️ Ошибка получения деталей коллекции %d: %v", collection.ID, err)
			continue
		}

		// Запоминаем всех персонажей
		for _, character := range details.Data.Characters {
			key := fmt.Sprintf("%d:%d", collection.ID, character.ID)
			s.knownCharacters[key] = true
		}
	}

	s.log("📋 Инициализировано: %d коллекций, %d персонажей",
		len(s.knownCollections), len(s.knownCharacters))

	return nil
}

// monitorLoop основной цикл мониторинга
func (s *SnipeMonitor) monitorLoop() {
	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-s.ctx.Done():
			return
		case <-ticker.C:
			if err := s.checkForNewItems(); err != nil {
				s.log("❌ Ошибка проверки: %v", err)
			}
		}
	}
}

// checkForNewItems проверяет новые коллекции и персонажи
func (s *SnipeMonitor) checkForNewItems() error {
	// Получаем кешированный токен (без API проверки)
	token, err := s.tokenCallback(s.config.Name)
	if err != nil {
		return fmt.Errorf("ошибка получения токена: %v", err)
	}

	collections, err := s.apiClient.GetCollections(token)
	tokenWasRefreshed := false
	if err != nil {
		// Проверяем, является ли это ошибкой токена
		if tokenErr, ok := err.(*TokenError); ok {
			s.log("🔑 Ошибка токена при мониторинге: %v", tokenErr)
			// Пытаемся обновить токен
			newToken, refreshErr := s.tokenRefreshCallback(s.config.Name, tokenErr.StatusCode)
			if refreshErr != nil {
				return fmt.Errorf("ошибка обновления токена: %v", refreshErr)
			}
			tokenWasRefreshed = true
			token = newToken // Обновляем токен для дальнейшего использования
			// Повторяем запрос с новым токеном
			collections, err = s.apiClient.GetCollections(newToken)
			if err != nil {
				return fmt.Errorf("ошибка получения коллекций после обновления токена: %v", err)
			}
		} else {
			return fmt.Errorf("ошибка получения коллекций: %v", err)
		}
	}

	s.mutex.Lock()
	defer s.mutex.Unlock()

	// Если токен был обновлен и состояние пустое, выполняем повторную инициализацию
	if tokenWasRefreshed && len(s.knownCollections) == 0 {
		s.log("🔄 Токен был обновлен и состояние пустое, выполняем повторную инициализацию...")

		// Запоминаем все существующие коллекции как известные (не новые)
		for _, collection := range collections.Data {
			s.knownCollections[collection.ID] = true

			// Получаем детали коллекции для запоминания персонажей
			details, err := s.apiClient.GetCollectionDetails(token, collection.ID)
			if err != nil {
				s.log("⚠️ Ошибка получения деталей коллекции %d при реинициализации: %v", collection.ID, err)
				continue
			}

			// Запоминаем всех персонажей
			for _, character := range details.Data.Characters {
				key := fmt.Sprintf("%d:%d", collection.ID, character.ID)
				s.knownCharacters[key] = true
			}
		}

		s.log("🔄 Реинициализация завершена: %d коллекций, %d персонажей помечены как известные",
			len(s.knownCollections), len(s.knownCharacters))

		// После реинициализации не проверяем коллекции как новые
		return nil
	}

	// Проверяем новые коллекции
	for _, collection := range collections.Data {
		if !s.knownCollections[collection.ID] {
			s.log("🆕 Найдена новая коллекция: %d - %s", collection.ID, collection.Title)
			s.knownCollections[collection.ID] = true

			// Проверяем коллекцию на соответствие фильтрам
			if err := s.checkCollection(collection); err != nil {
				s.log("⚠️ Ошибка проверки коллекции %d: %v", collection.ID, err)
			}
		}

		// Проверяем новых персонажей в существующих коллекциях
		if err := s.checkCollectionForNewCharacters(collection.ID); err != nil {
			s.log("⚠️ Ошибка проверки персонажей коллекции %d: %v", collection.ID, err)
		}
	}

	return nil
}

// checkCollection проверяет коллекцию на соответствие фильтрам
func (s *SnipeMonitor) checkCollection(collection Collection) error {
	// Получаем кешированный токен (без API проверки)
	token, err := s.tokenCallback(s.config.Name)
	if err != nil {
		return fmt.Errorf("ошибка получения токена: %v", err)
	}

	details, err := s.apiClient.GetCollectionDetails(token, collection.ID)
	if err != nil {
		// Если ошибка авторизации, токен будет обновлен автоматически в buyer.go
		return fmt.Errorf("ошибка получения деталей коллекции: %v", err)
	}

	// Проверяем фильтр по словам
	if !s.matchesWordFilter(collection.Title) {
		s.log("🚫 Коллекция %d не прошла фильтр по словам: %s", collection.ID, collection.Title)
		return nil
	}

	// Проверяем каждого персонажа
	for _, character := range details.Data.Characters {
		key := fmt.Sprintf("%d:%d", collection.ID, character.ID)
		s.knownCharacters[key] = true

		if s.matchesFilters(character) {
			s.log("✅ Найден подходящий персонаж: %s (ID: %d, Цена: %d, Supply: %d)",
				character.Name, character.ID, character.Price, character.Supply)

			// Логируем найденную коллекцию в файл
			if err := s.collectionLogger.LogFoundCollection(collection, character, s.config.Name); err != nil {
				s.log("⚠️ Ошибка сохранения коллекции в лог: %v", err)
			} else {
				s.log("💾 Коллекция сохранена в лог файл")
			}

			// Отправляем запрос на покупку
			request := PurchaseRequest{
				CollectionID: collection.ID,
				CharacterID:  character.ID,
				Price:        character.Price,
				Supply:       character.Supply,
				Name:         character.Name,
			}

			if err := s.purchaseCallback(request); err != nil {
				s.log("❌ Ошибка покупки: %v", err)
			}
		}
	}

	return nil
}

// checkCollectionForNewCharacters проверяет новых персонажей в коллекции
func (s *SnipeMonitor) checkCollectionForNewCharacters(collectionID int) error {
	// Получаем кешированный токен (без API проверки)
	token, err := s.tokenCallback(s.config.Name)
	if err != nil {
		return fmt.Errorf("ошибка получения токена: %v", err)
	}

	details, err := s.apiClient.GetCollectionDetails(token, collectionID)
	if err != nil {
		// Если ошибка авторизации, токен будет обновлен автоматически в buyer.go
		return fmt.Errorf("ошибка получения деталей коллекции: %v", err)
	}

	for _, character := range details.Data.Characters {
		key := fmt.Sprintf("%d:%d", collectionID, character.ID)

		if !s.knownCharacters[key] {
			s.log("🆕 Найден новый персонаж: %s в коллекции %d", character.Name, collectionID)
			s.knownCharacters[key] = true

			// Проверяем фильтр по словам для названия коллекции
			if !s.matchesWordFilter(details.Data.Collection.Title) {
				s.log("🚫 Персонаж %d не прошел фильтр по словам коллекции: %s",
					character.ID, details.Data.Collection.Title)
				continue
			}

			if s.matchesFilters(character) {
				s.log("✅ Найден подходящий новый персонаж: %s (ID: %d, Цена: %d, Supply: %d)",
					character.Name, character.ID, character.Price, character.Supply)

				// Логируем найденную коллекцию в файл
				if err := s.collectionLogger.LogFoundCollection(details.Data.Collection, character, s.config.Name); err != nil {
					s.log("⚠️ Ошибка сохранения коллекции в лог: %v", err)
				} else {
					s.log("💾 Коллекция сохранена в лог файл")
				}

				// Отправляем запрос на покупку
				request := PurchaseRequest{
					CollectionID: collectionID,
					CharacterID:  character.ID,
					Price:        character.Price,
					Supply:       character.Supply,
					Name:         character.Name,
				}

				if err := s.purchaseCallback(request); err != nil {
					s.log("❌ Ошибка покупки: %v", err)
				}
			}
		}
	}

	return nil
}

// matchesWordFilter проверяет соответствие фильтру по словам
func (s *SnipeMonitor) matchesWordFilter(title string) bool {
	// Если фильтр не задан, пропускаем всё
	if len(s.config.SnipeMonitor.WordFilter) == 0 {
		return true
	}

	titleLower := strings.ToLower(title)

	// Проверяем наличие хотя бы одного слова из фильтра
	for _, word := range s.config.SnipeMonitor.WordFilter {
		if strings.Contains(titleLower, strings.ToLower(word)) {
			return true
		}
	}

	return false
}

// matchesFilters проверяет соответствие персонажа всем фильтрам
func (s *SnipeMonitor) matchesFilters(character Character) bool {
	// Проверяем диапазон количества
	if s.config.SnipeMonitor.SupplyRange != nil {
		if character.Supply < s.config.SnipeMonitor.SupplyRange.Min ||
			character.Supply > s.config.SnipeMonitor.SupplyRange.Max {
			s.log("🚫 Персонаж %s не прошел фильтр по supply: %d (нужно: %d-%d)",
				character.Name, character.Supply,
				s.config.SnipeMonitor.SupplyRange.Min, s.config.SnipeMonitor.SupplyRange.Max)
			return false
		}
	}

	// Проверяем диапазон цен
	if s.config.SnipeMonitor.PriceRange != nil {
		if character.Price < s.config.SnipeMonitor.PriceRange.Min ||
			character.Price > s.config.SnipeMonitor.PriceRange.Max {
			s.log("🚫 Персонаж %s не прошел фильтр по цене: %d (нужно: %d-%d)",
				character.Name, character.Price,
				s.config.SnipeMonitor.PriceRange.Min, s.config.SnipeMonitor.PriceRange.Max)
			return false
		}
	}

	return true
}

// log выводит лог с префиксом
func (s *SnipeMonitor) log(format string, args ...interface{}) {
	message := fmt.Sprintf(format, args...)
	log.Printf("%s %s", s.logPrefix, message)
}
