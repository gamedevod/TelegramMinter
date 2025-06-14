package monitor

import (
	"encoding/json"
	"fmt"
	"os"
	"sync"
	"time"
)

// FoundCollection структура для сохранения найденной коллекции
type FoundCollection struct {
	ID          int       `json:"id"`
	Name        string    `json:"name"`
	CharacterID int       `json:"character_id"`
	Supply      int       `json:"supply"`
	PriceTON    float64   `json:"price_ton"`
	PriceNano   int       `json:"price_nano"`
	FoundAt     time.Time `json:"found_at"`
	AccountName string    `json:"account_name"`
}

// CollectionLogger логгер для сохранения найденных коллекций
type CollectionLogger struct {
	filename string
	mutex    sync.Mutex
}

// NewCollectionLogger создает новый логгер коллекций
func NewCollectionLogger(filename string) *CollectionLogger {
	return &CollectionLogger{
		filename: filename,
	}
}

// LogFoundCollection сохраняет найденную коллекцию в файл
func (cl *CollectionLogger) LogFoundCollection(collection Collection, character Character, accountName string) error {
	cl.mutex.Lock()
	defer cl.mutex.Unlock()

	// Конвертируем цену из нанотонов в TON
	priceTON := float64(character.Price) / 1000000000.0

	foundCollection := FoundCollection{
		ID:          collection.ID,
		Name:        collection.Title,
		CharacterID: character.ID,
		Supply:      character.Supply,
		PriceTON:    priceTON,
		PriceNano:   character.Price,
		FoundAt:     time.Now(),
		AccountName: accountName,
	}

	// Читаем существующие данные
	var collections []FoundCollection
	if data, err := os.ReadFile(cl.filename); err == nil {
		json.Unmarshal(data, &collections)
	}

	// Добавляем новую коллекцию
	collections = append(collections, foundCollection)

	// Сохраняем обратно в файл
	data, err := json.MarshalIndent(collections, "", "  ")
	if err != nil {
		return fmt.Errorf("ошибка сериализации JSON: %v", err)
	}

	if err := os.WriteFile(cl.filename, data, 0644); err != nil {
		return fmt.Errorf("ошибка записи в файл: %v", err)
	}

	return nil
}

// GetFoundCollections возвращает все найденные коллекции
func (cl *CollectionLogger) GetFoundCollections() ([]FoundCollection, error) {
	cl.mutex.Lock()
	defer cl.mutex.Unlock()

	var collections []FoundCollection
	data, err := os.ReadFile(cl.filename)
	if err != nil {
		if os.IsNotExist(err) {
			return collections, nil // Возвращаем пустой массив если файл не существует
		}
		return nil, fmt.Errorf("ошибка чтения файла: %v", err)
	}

	if err := json.Unmarshal(data, &collections); err != nil {
		return nil, fmt.Errorf("ошибка парсинга JSON: %v", err)
	}

	return collections, nil
}

// GetCollectionCount возвращает количество найденных коллекций
func (cl *CollectionLogger) GetCollectionCount() int {
	collections, err := cl.GetFoundCollections()
	if err != nil {
		return 0
	}
	return len(collections)
}
