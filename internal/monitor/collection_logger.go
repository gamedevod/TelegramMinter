package monitor

import (
	"encoding/json"
	"fmt"
	"os"
	"sync"
	"time"
)

// FoundCollection structure for saving found collection
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

// CollectionLogger logger for saving found collections
type CollectionLogger struct {
	filename string
	mutex    sync.Mutex
}

// NewCollectionLogger creates a new collection logger
func NewCollectionLogger(filename string) *CollectionLogger {
	return &CollectionLogger{
		filename: filename,
	}
}

// LogFoundCollection saves found collection to file
func (cl *CollectionLogger) LogFoundCollection(collection Collection, character Character, accountName string) error {
	cl.mutex.Lock()
	defer cl.mutex.Unlock()

	// Convert price from nanotons to TON
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

	// Read existing data
	var collections []FoundCollection
	if data, err := os.ReadFile(cl.filename); err == nil {
		json.Unmarshal(data, &collections)
	}

	// Add new collection
	collections = append(collections, foundCollection)

	// Save back to file
	data, err := json.MarshalIndent(collections, "", "  ")
	if err != nil {
		return fmt.Errorf("JSON serialization error: %v", err)
	}

	if err := os.WriteFile(cl.filename, data, 0644); err != nil {
		return fmt.Errorf("file write error: %v", err)
	}

	return nil
}

// GetFoundCollections returns all found collections
func (cl *CollectionLogger) GetFoundCollections() ([]FoundCollection, error) {
	cl.mutex.Lock()
	defer cl.mutex.Unlock()

	var collections []FoundCollection
	data, err := os.ReadFile(cl.filename)
	if err != nil {
		if os.IsNotExist(err) {
			return collections, nil // Return empty array if file doesn't exist
		}
		return nil, fmt.Errorf("file read error: %v", err)
	}

	if err := json.Unmarshal(data, &collections); err != nil {
		return nil, fmt.Errorf("JSON parsing error: %v", err)
	}

	return collections, nil
}

// GetCollectionCount returns the number of found collections
func (cl *CollectionLogger) GetCollectionCount() int {
	collections, err := cl.GetFoundCollections()
	if err != nil {
		return 0
	}
	return len(collections)
}
