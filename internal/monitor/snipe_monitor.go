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

// PurchaseRequest represents a purchase request structure
type PurchaseRequest struct {
	CollectionID int
	CharacterID  int
	Price        int
	Supply       int
	Name         string
}

// PurchaseCallback is a callback function for purchase
type PurchaseCallback func(request PurchaseRequest) error

// TokenCallback is a callback function for getting a valid token
type TokenCallback func(accountName string) (string, error)

// TokenRefreshCallback is a callback function for refreshing token on error
type TokenRefreshCallback func(accountName string, statusCode int) (string, error)

// SnipeMonitor represents snipe monitor structure
type SnipeMonitor struct {
	config               *config.Account
	apiClient            *APIClient
	httpClient           *client.HTTPClient
	purchaseCallback     PurchaseCallback
	tokenCallback        TokenCallback
	tokenRefreshCallback TokenRefreshCallback

	// State
	knownCollections map[int]bool    // IDs of known collections
	knownCharacters  map[string]bool // "collectionID:characterID" of known characters
	mutex            sync.RWMutex

	// Lifecycle management
	ctx    context.Context
	cancel context.CancelFunc

	// Logging
	logPrefix        string
	collectionLogger *CollectionLogger
}

// NewSnipeMonitor creates a new snipe monitor
func NewSnipeMonitor(account *config.Account, httpClient *client.HTTPClient, purchaseCallback PurchaseCallback, tokenCallback TokenCallback, tokenRefreshCallback TokenRefreshCallback) *SnipeMonitor {
	ctx, cancel := context.WithCancel(context.Background())

	// Create filename for collection logs
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

// Start launches the snipe monitor
func (s *SnipeMonitor) Start() error {
	if s.config.SnipeMonitor == nil || !s.config.SnipeMonitor.Enabled {
		return fmt.Errorf("snipe monitor is not enabled")
	}

	if s.config.AuthToken == "" {
		return fmt.Errorf("authorization token is missing")
	}

	s.log("🎯 Snipe monitor started")
	s.log("📊 Settings:")
	if s.config.SnipeMonitor.SupplyRange != nil {
		s.log("   Supply: %d - %d", s.config.SnipeMonitor.SupplyRange.Min, s.config.SnipeMonitor.SupplyRange.Max)
	}
	if s.config.SnipeMonitor.PriceRange != nil {
		s.log("   Price: %d - %d nanoton", s.config.SnipeMonitor.PriceRange.Min, s.config.SnipeMonitor.PriceRange.Max)
	}
	if len(s.config.SnipeMonitor.WordFilter) > 0 {
		s.log("   Word filter: %v", s.config.SnipeMonitor.WordFilter)
	}

	// Initialize state - get current collections
	if err := s.initializeState(); err != nil {
		s.log("⚠️ State initialization error: %v", err)
	}

	// Start main monitoring loop
	go s.monitorLoop()

	return nil
}

// Stop stops the snipe monitor
func (s *SnipeMonitor) Stop() {
	s.log("🛑 Stopping snipe monitor")
	s.cancel()
}

// GetAccountName returns the account name associated with this snipe monitor
func (s *SnipeMonitor) GetAccountName() string {
	return s.config.Name
}

// initializeState initializes monitor state
func (s *SnipeMonitor) initializeState() error {
	// Get valid token
	token, err := s.tokenCallback(s.config.Name)
	if err != nil {
		return fmt.Errorf("error getting token: %v", err)
	}

	collections, err := s.apiClient.GetCollections(token)
	if err != nil {
		// Check if this is a token error
		if tokenErr, ok := err.(*TokenError); ok {
			s.log("🔑 Token error during initialization: %v", tokenErr)
			// Try to refresh token
			newToken, refreshErr := s.tokenRefreshCallback(s.config.Name, tokenErr.StatusCode)
			if refreshErr != nil {
				return fmt.Errorf("error refreshing token: %v", refreshErr)
			}
			token = newToken // Update token for further use
			// Retry request with new token
			collections, err = s.apiClient.GetCollections(newToken)
			if err != nil {
				return fmt.Errorf("error getting collections after token refresh: %v", err)
			}
		} else {
			return fmt.Errorf("error getting collections: %v", err)
		}
	}

	s.mutex.Lock()
	defer s.mutex.Unlock()

	// Remember all existing collections
	for _, collection := range collections.Data {
		s.knownCollections[collection.ID] = true

		// Get collection details to remember characters
		details, err := s.apiClient.GetCollectionDetails(token, collection.ID)
		if err != nil {
			s.log("⚠️ Error getting collection details %d: %v", collection.ID, err)
			continue
		}

		// Remember all characters
		for _, character := range details.Data.Characters {
			key := fmt.Sprintf("%d:%d", collection.ID, character.ID)
			s.knownCharacters[key] = true
		}
	}

	s.log("📋 Initialized: %d collections, %d characters",
		len(s.knownCollections), len(s.knownCharacters))

	return nil
}

// monitorLoop is the main monitoring loop
func (s *SnipeMonitor) monitorLoop() {
	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-s.ctx.Done():
			return
		case <-ticker.C:
			if err := s.checkForNewItems(); err != nil {
				s.log("❌ Check error: %v", err)
			}
		}
	}
}

// checkForNewItems checks for new collections and characters
func (s *SnipeMonitor) checkForNewItems() error {
	// Get cached token (without API verification)
	token, err := s.tokenCallback(s.config.Name)
	if err != nil {
		return fmt.Errorf("error getting token: %v", err)
	}

	collections, err := s.apiClient.GetCollections(token)
	tokenWasRefreshed := false
	if err != nil {
		// Check if this is a token error
		if tokenErr, ok := err.(*TokenError); ok {
			s.log("�� Token error during monitoring: %v", tokenErr)
			// Try to refresh token
			newToken, refreshErr := s.tokenRefreshCallback(s.config.Name, tokenErr.StatusCode)
			if refreshErr != nil {
				return fmt.Errorf("error refreshing token: %v", refreshErr)
			}
			tokenWasRefreshed = true
			token = newToken // Update token for further use
			// Retry request with new token
			collections, err = s.apiClient.GetCollections(newToken)
			if err != nil {
				return fmt.Errorf("error getting collections after token refresh: %v", err)
			}
		} else {
			return fmt.Errorf("error getting collections: %v", err)
		}
	}

	s.mutex.Lock()
	defer s.mutex.Unlock()

	// If token was refreshed and state is empty, perform reinitialization
	if tokenWasRefreshed && len(s.knownCollections) == 0 {
		s.log("🔄 Token was refreshed and state is empty, performing reinitialization...")

		// Remember all existing collections as known (not new)
		for _, collection := range collections.Data {
			s.knownCollections[collection.ID] = true

			// Get collection details to remember characters
			details, err := s.apiClient.GetCollectionDetails(token, collection.ID)
			if err != nil {
				s.log("⚠️ Error getting collection details %d during reinitialization: %v", collection.ID, err)
				continue
			}

			// Remember all characters
			for _, character := range details.Data.Characters {
				key := fmt.Sprintf("%d:%d", collection.ID, character.ID)
				s.knownCharacters[key] = true
			}
		}

		s.log("🔄 Reinitialization completed: %d collections, %d characters marked as known",
			len(s.knownCollections), len(s.knownCharacters))

		// After reinitialization, do not check collections as new
		return nil
	}

	// Check for new collections
	for _, collection := range collections.Data {
		if !s.knownCollections[collection.ID] {
			s.log("🆕 New collection found: %d - %s", collection.ID, collection.Title)
			s.knownCollections[collection.ID] = true

			// Check collection against filters
			if err := s.checkCollection(collection); err != nil {
				s.log("⚠️ Collection check error %d: %v", collection.ID, err)
			}
		}

		// Check for new characters in existing collections
		if err := s.checkCollectionForNewCharacters(collection.ID); err != nil {
			s.log("⚠️ Character check error in collection %d: %v", collection.ID, err)
		}
	}

	return nil
}

// checkCollection checks collection against filters
func (s *SnipeMonitor) checkCollection(collection Collection) error {
	// Get cached token (without API verification)
	token, err := s.tokenCallback(s.config.Name)
	if err != nil {
		return fmt.Errorf("error getting token: %v", err)
	}

	details, err := s.apiClient.GetCollectionDetails(token, collection.ID)
	if err != nil {
		// If authorization error, token will be refreshed automatically in buyer.go
		return fmt.Errorf("error getting collection details: %v", err)
	}

	// Check word filter
	if !s.matchesWordFilter(collection.Title) {
		s.log("🚫 Collection %d did not pass word filter: %s", collection.ID, collection.Title)
		return nil
	}

	// Check each character
	for _, character := range details.Data.Characters {
		key := fmt.Sprintf("%d:%d", collection.ID, character.ID)
		s.knownCharacters[key] = true

		if s.matchesFilters(character) {
			s.log("✅ Suitable character found: %s (ID: %d, Price: %d, Supply: %d)",
				character.Name, character.ID, character.Price, character.Supply)

			// Log found collection to file
			if err := s.collectionLogger.LogFoundCollection(collection, character, s.config.Name); err != nil {
				s.log("⚠️ Error saving collection to log: %v", err)
			} else {
				s.log("💾 Collection saved to log file")
			}

			// Send purchase request
			request := PurchaseRequest{
				CollectionID: collection.ID,
				CharacterID:  character.ID,
				Price:        character.Price,
				Supply:       character.Supply,
				Name:         character.Name,
			}

			if err := s.purchaseCallback(request); err != nil {
				s.log("❌ Purchase error: %v", err)
			}
		}
	}

	return nil
}

// checkCollectionForNewCharacters checks for new characters in collection
func (s *SnipeMonitor) checkCollectionForNewCharacters(collectionID int) error {
	// Get cached token (without API verification)
	token, err := s.tokenCallback(s.config.Name)
	if err != nil {
		return fmt.Errorf("error getting token: %v", err)
	}

	details, err := s.apiClient.GetCollectionDetails(token, collectionID)
	if err != nil {
		// If authorization error, token will be refreshed automatically in buyer.go
		return fmt.Errorf("error getting collection details: %v", err)
	}

	for _, character := range details.Data.Characters {
		key := fmt.Sprintf("%d:%d", collectionID, character.ID)

		if !s.knownCharacters[key] {
			s.log("🆕 New character found: %s in collection %d", character.Name, collectionID)
			s.knownCharacters[key] = true

			// Check word filter for collection title
			if !s.matchesWordFilter(details.Data.Collection.Title) {
				s.log("🚫 Character %d did not pass collection word filter: %s",
					character.ID, details.Data.Collection.Title)
				continue
			}

			if s.matchesFilters(character) {
				s.log("✅ Suitable new character found: %s (ID: %d, Price: %d, Supply: %d)",
					character.Name, character.ID, character.Price, character.Supply)

				// Log found collection to file
				if err := s.collectionLogger.LogFoundCollection(details.Data.Collection, character, s.config.Name); err != nil {
					s.log("⚠️ Error saving collection to log: %v", err)
				} else {
					s.log("💾 Collection saved to log file")
				}

				// Send purchase request
				request := PurchaseRequest{
					CollectionID: collectionID,
					CharacterID:  character.ID,
					Price:        character.Price,
					Supply:       character.Supply,
					Name:         character.Name,
				}

				if err := s.purchaseCallback(request); err != nil {
					s.log("❌ Purchase error: %v", err)
				}
			}
		}
	}

	return nil
}

// matchesWordFilter checks against word filter
func (s *SnipeMonitor) matchesWordFilter(title string) bool {
	// If filter not specified, skip all
	if len(s.config.SnipeMonitor.WordFilter) == 0 {
		return true
	}

	titleLower := strings.ToLower(title)

	// Check for presence of at least one word from filter
	for _, word := range s.config.SnipeMonitor.WordFilter {
		if strings.Contains(titleLower, strings.ToLower(word)) {
			return true
		}
	}

	return false
}

// matchesFilters checks against all filters
func (s *SnipeMonitor) matchesFilters(character Character) bool {
	// Check quantity range
	if s.config.SnipeMonitor.SupplyRange != nil {
		if character.Supply < s.config.SnipeMonitor.SupplyRange.Min ||
			character.Supply > s.config.SnipeMonitor.SupplyRange.Max {
			s.log("🚫 Character %s did not pass supply filter: %d (need: %d-%d)",
				character.Name, character.Supply,
				s.config.SnipeMonitor.SupplyRange.Min, s.config.SnipeMonitor.SupplyRange.Max)
			return false
		}
	}

	// Check price range
	if s.config.SnipeMonitor.PriceRange != nil {
		if character.Price < s.config.SnipeMonitor.PriceRange.Min ||
			character.Price > s.config.SnipeMonitor.PriceRange.Max {
			s.log("🚫 Character %s did not pass price filter: %d (need: %d-%d)",
				character.Name, character.Price,
				s.config.SnipeMonitor.PriceRange.Min, s.config.SnipeMonitor.PriceRange.Max)
			return false
		}
	}

	return true
}

// log outputs log with prefix
func (s *SnipeMonitor) log(format string, args ...interface{}) {
	message := fmt.Sprintf(format, args...)
	log.Printf("%s %s", s.logPrefix, message)
}
