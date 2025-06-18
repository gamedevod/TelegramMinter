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
	"stickersbot/internal/storage"
	"stickersbot/internal/types"
)

// AccountWorker structure for working with individual account
type AccountWorker struct {
	client           *client.HTTPClient
	account          config.Account
	testMode         bool
	testAddr         string
	workerID         int
	transactionCount int          // Counter of successful transactions
	isActive         bool         // Account activity flag
	mu               sync.RWMutex // Mutex for safe access to counters
}

// BuyerService service for purchasing stickers
type BuyerService struct {
	client         *client.HTTPClient
	config         *config.Config
	statistics     *types.Statistics
	isRunning      bool
	isStopping     bool // Flag to indicate stopping in progress
	cancel         context.CancelFunc
	mu             sync.RWMutex
	logChan        chan string
	transactionLog *os.File // File for transaction logging

	// Snipe monitors
	snipeMonitors []*monitor.SnipeMonitor

	// Token manager
	tokenManager *TokenManager
	// Proxy/token storage
	tokenStorage *storage.TokenStorage

	// Snipe transaction counters per account
	snipeTransactionCounters map[string]int // Account name -> transaction count
	snipeCountersMu          sync.RWMutex   // Mutex for snipe counters

	// Active accounts tracking
	activeAccounts   map[string]bool // Account name -> is active
	totalAccounts    int             // Total number of accounts
	activeAccountsMu sync.RWMutex    // Mutex for active accounts
}

// NewBuyerService creates a new purchase service
func NewBuyerService(cfg *config.Config, ts *storage.TokenStorage) *BuyerService {
	// Create file for transaction logging
	logFile, err := os.OpenFile("transactions.log", os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		fmt.Printf("⚠️ Failed to create transaction log file: %v\n", err)
		logFile = nil
	}

	return &BuyerService{
		client:                   client.New(),
		config:                   cfg,
		statistics:               &types.Statistics{},
		logChan:                  make(chan string, 1000),
		transactionLog:           logFile,
		tokenManager:             NewTokenManager(cfg, ts),
		tokenStorage:             ts,
		snipeTransactionCounters: make(map[string]int),
		activeAccounts:           make(map[string]bool),
		totalAccounts:            0,
	}
}

// Start launches the sticker purchase process
func (bs *BuyerService) Start() error {
	bs.mu.Lock()
	defer bs.mu.Unlock()

	if bs.isRunning {
		return fmt.Errorf("service is already running")
	}

	if !bs.config.IsValid() {
		return fmt.Errorf("invalid configuration: check accounts")
	}

	ctx, cancel := context.WithCancel(context.Background())
	bs.cancel = cancel
	bs.isRunning = true

	// Recreate token manager with current storage reference
	bs.tokenManager = NewTokenManager(bs.config, bs.tokenStorage)

	// Initialize token cache
	bs.tokenManager.InitializeTokens()

	// Start preventive token refresh every 30 minutes
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

	// Initialize statistics
	bs.statistics = &types.Statistics{
		StartTime: time.Now(),
	}

	bs.logChan <- "🚀 Starting sticker purchase..."
	bs.logChan <- fmt.Sprintf("📊 Accounts: %d", len(bs.config.Accounts))

	// Initialize tokens from configuration
	bs.logChan <- "🔍 Initializing authorization tokens..."

	// Count total number of threads
	totalThreads := 0
	for _, account := range bs.config.Accounts {
		totalThreads += account.Threads
	}
	bs.logChan <- fmt.Sprintf("🔄 Total number of threads: %d", totalThreads)

	if bs.config.TestMode {
		bs.logChan <- fmt.Sprintf("🧪 TEST MODE: payments will be sent to %s", bs.config.TestAddress)
	} else {
		bs.logChan <- "⚠️ PRODUCTION MODE: payments will be sent to addresses from API"
	}

	// Initialize active accounts tracking
	bs.activeAccountsMu.Lock()
	bs.totalAccounts = len(bs.config.Accounts)
	for _, account := range bs.config.Accounts {
		// Only mark accounts as active if they will actually run (not snipe-only or disabled)
		if account.SnipeMonitor == nil || !account.SnipeMonitor.Enabled {
			bs.activeAccounts[account.Name] = true
		} else {
			// For snipe accounts, they are active until they reach transaction limit
			bs.activeAccounts[account.Name] = true
		}
	}
	bs.activeAccountsMu.Unlock()

	// Launch workers for each account
	var wg sync.WaitGroup
	workerCounter := 0

	for accountIndex, account := range bs.config.Accounts {
		bs.logChan <- fmt.Sprintf("🎯 Account '%s': Collection: %d, Character: %d, Currency: %s, Amount: %d, Threads: %d",
			account.Name, account.Collection, account.Character, account.Currency, account.Count, account.Threads)

		if account.SeedPhrase != "" {
			bs.logChan <- fmt.Sprintf("🔐 Account '%s': TON wallet configured", account.Name)
		} else {
			bs.logChan <- fmt.Sprintf("⚠️ Account '%s': TON wallet NOT configured", account.Name)
		}

		// Check if snipe monitor needs to be launched for this account
		if account.SnipeMonitor != nil && account.SnipeMonitor.Enabled {
			bs.logChan <- fmt.Sprintf("🎯 Account '%s': Launching snipe monitor", account.Name)

			// Create purchase callback function
			purchaseCallback := bs.createPurchaseCallback(&account)

			// Create token retrieval callback
			tokenCallback := func(accountName string) (string, error) {
				return bs.tokenManager.GetValidToken(accountName)
			}

			// Create token refresh callback
			tokenRefreshCallback := func(accountName string, statusCode int) (string, error) {
				return bs.tokenManager.RefreshTokenOnError(accountName, statusCode)
			}

			// Create HTTP client with account-specific proxy settings
			monitorClient, err := client.NewForAccount(account.UseProxy, account.ProxyURL)
			if err != nil {
				bs.logChan <- fmt.Sprintf("❌ Error creating HTTP client for snipe monitor '%s': %v", account.Name, err)
				continue
			}

			// Create and launch snipe monitor
			snipeMonitor := monitor.NewSnipeMonitor(&account, monitorClient, purchaseCallback, tokenCallback, tokenRefreshCallback)
			bs.snipeMonitors = append(bs.snipeMonitors, snipeMonitor)

			if err := snipeMonitor.Start(); err != nil {
				bs.logChan <- fmt.Sprintf("❌ Error launching snipe monitor for account '%s': %v", account.Name, err)
			}
		} else {
			// Launch regular threads for this account
			for i := 0; i < account.Threads; i++ {
				wg.Add(1)
				workerCounter++

				accountWorker, err := createAccountWorker(account, bs.config.TestMode, bs.config.TestAddress, workerCounter)
				if err != nil {
					bs.logChan <- fmt.Sprintf("❌ Error creating account worker for account '%s': %v", account.Name, err)
					continue
				}

				go bs.accountWorker(ctx, &wg, accountWorker, accountIndex+1)
			}
		}
	}

	// Launch goroutine for statistics update
	go bs.updateStatistics(ctx)

	// Wait for completion in separate goroutine
	go func() {
		wg.Wait()
		bs.mu.Lock()
		bs.isRunning = false
		bs.mu.Unlock()
		bs.logChan <- "✅ All threads completed"
	}()

	return nil
}

// accountWorker executes purchases for a specific account
func (bs *BuyerService) accountWorker(ctx context.Context, wg *sync.WaitGroup, worker *AccountWorker, accountNum int) {
	defer wg.Done()

	bs.logChan <- fmt.Sprintf("🔄 Thread %d started for account %d '%s'", worker.workerID, accountNum, worker.account.Name)

	for {
		select {
		case <-ctx.Done():
			bs.logChan <- fmt.Sprintf("🛑 Thread %d stopped", worker.workerID)
			return
		default:
			// Check if service is stopping
			bs.mu.RLock()
			stopping := bs.isStopping
			bs.mu.RUnlock()

			if stopping {
				bs.logChan <- fmt.Sprintf("🛑 Thread %d stopping gracefully", worker.workerID)
				return
			}

			// Check if account is active
			worker.mu.RLock()
			isActive := worker.isActive
			worker.mu.RUnlock()

			if !isActive {
				bs.logChan <- fmt.Sprintf("🛑 Thread %d inactive (reached transaction limit)", worker.workerID)
				return
			}

			bs.performAccountBuy(worker, accountNum)
			delay := time.Duration(worker.account.PurchaseDelayMs)
			if delay <= 0 {
				delay = 100
			}
			time.Sleep(delay * time.Millisecond)
		}
	}
}

// performAccountBuy executes purchase for a specific account
func (bs *BuyerService) performAccountBuy(worker *AccountWorker, accountNum int) {
	// Get cached token (without API check)
	bearerToken, err := bs.tokenManager.GetValidToken(worker.account.Name)
	if err != nil {
		bs.mu.Lock()
		bs.statistics.FailedRequests++
		bs.mu.Unlock()
		bs.logChan <- fmt.Sprintf("❌ Thread %d (Account %d '%s'): Token retrieval error: %v",
			worker.workerID, accountNum, worker.account.Name, err)
		return
	}

	// Execute purchase request
	resp, err := bs.makeOrderRequest(worker.account, bearerToken)
	if err != nil {
		bs.mu.Lock()
		bs.statistics.FailedRequests++
		bs.mu.Unlock()
		bs.logChan <- fmt.Sprintf("❌ Thread %d (Account %d '%s'): Request error: %v",
			worker.workerID, accountNum, worker.account.Name, err)
		return
	}

	// Check response status
	if resp.StatusCode == 401 || resp.StatusCode == 403 {
		// Token expired, try to refresh and retry request
		bs.logChan <- fmt.Sprintf("🔄 Thread %d (Account %d '%s'): Token expired (status %d), refreshing...",
			worker.workerID, accountNum, worker.account.Name, resp.StatusCode)

		newToken, err := bs.tokenManager.RefreshTokenOnError(worker.account.Name, resp.StatusCode)
		if err != nil {
			bs.mu.Lock()
			bs.statistics.FailedRequests++
			bs.mu.Unlock()
			bs.logChan <- fmt.Sprintf("❌ Thread %d (Account %d '%s'): Token refresh error: %v",
				worker.workerID, accountNum, worker.account.Name, err)
			return
		}

		// Retry request with new token
		resp2, err := bs.makeOrderRequest(worker.account, newToken)
		if err != nil {
			bs.mu.Lock()
			bs.statistics.FailedRequests++
			bs.mu.Unlock()
			bs.logChan <- fmt.Sprintf("❌ Thread %d (Account %d '%s'): Retry request error: %v",
				worker.workerID, accountNum, worker.account.Name, err)
			return
		}
		resp = resp2 // Use new response
	}

	// Log server response
	bs.logChan <- fmt.Sprintf("📡 Thread %d (Account %d '%s'): Status %d", worker.workerID, accountNum, worker.account.Name, resp.StatusCode)
	bs.logChan <- fmt.Sprintf("📄 Thread %d (Account %d '%s'): Response - %s", worker.workerID, accountNum, worker.account.Name, resp.Body)

	if resp.IsTokenError {
		bs.mu.Lock()
		bs.statistics.FailedRequests++
		bs.statistics.InvalidTokens++
		bs.mu.Unlock()

		bs.logChan <- fmt.Sprintf("🔑 Thread %d (Account %d '%s'): Invalid authorization token! Refresh attempt...", worker.workerID, accountNum, worker.account.Name)

		// Try to refresh token
		newToken, err := bs.tokenManager.RefreshTokenOnError(worker.account.Name, resp.StatusCode)
		if err != nil {
			bs.logChan <- fmt.Sprintf("❌ Thread %d (Account %d '%s'): Token refresh error: %v", worker.workerID, accountNum, worker.account.Name, err)
			return
		}

		bs.logChan <- fmt.Sprintf("✅ Thread %d (Account %d '%s'): Token refreshed successfully, retrying request...", worker.workerID, accountNum, worker.account.Name)

		resp2, err := bs.makeOrderRequest(worker.account, newToken)
		if err != nil {
			bs.logChan <- fmt.Sprintf("❌ Thread %d (Account %d '%s'): Retry request error with new token: %v", worker.workerID, accountNum, worker.account.Name, err)
			return
		}

		resp = resp2 // Use new response
		bs.logChan <- fmt.Sprintf("🔄 Thread %d (Account %d '%s'): Retry request completed", worker.workerID, accountNum, worker.account.Name)
	}

	if !resp.Success {
		bs.mu.Lock()
		bs.statistics.FailedRequests++
		bs.mu.Unlock()

		bs.logChan <- fmt.Sprintf("⚠️ Thread %d (Account %d '%s'): Unsuccessful request (status %d)", worker.workerID, accountNum, worker.account.Name, resp.StatusCode)
	} else {
		// Successful request
		bs.mu.Lock()
		bs.statistics.SuccessRequests++
		bs.mu.Unlock()

		// Process transaction if it was sent
		if resp.TransactionSent && resp.TransactionResult != nil {
			// Update global statistics
			bs.mu.Lock()
			bs.statistics.SentTransactions++
			bs.mu.Unlock()

			// Update transaction counter for account
			worker.mu.Lock()
			worker.transactionCount++
			currentCount := worker.transactionCount

			// Check if account reached transaction limit
			if worker.account.MaxTransactions > 0 && currentCount >= worker.account.MaxTransactions {
				worker.isActive = false
				bs.logChan <- fmt.Sprintf("🛑 Account %d '%s' reached transaction limit (%d/%d) and will be stopped",
					accountNum, worker.account.Name, currentCount, worker.account.MaxTransactions)

				// Mark account as inactive in the service
				bs.setAccountInactive(worker.account.Name)
			}
			worker.mu.Unlock()

			// Log transaction information
			txResult := resp.TransactionResult
			bs.logChan <- fmt.Sprintf("💰 Thread %d (Account %d '%s'): Transaction sent!", worker.workerID, accountNum, worker.account.Name)
			bs.logChan <- fmt.Sprintf("   📤 From address: %s", txResult.FromAddress)
			bs.logChan <- fmt.Sprintf("   📥 To address: %s", txResult.ToAddress)
			bs.logChan <- fmt.Sprintf("   💰 Amount: %.9f TON", float64(txResult.Amount)/1000000000)
			bs.logChan <- fmt.Sprintf("   🔗 Order ID: %s", resp.OrderID)
			bs.logChan <- fmt.Sprintf("   🆔 Transaction ID: %s", txResult.TransactionID)
			bs.logChan <- fmt.Sprintf("   📊 Account transaction count: %d/%d", currentCount, worker.account.MaxTransactions)

			// Log transaction to file
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
			// Transaction attempt was made but failed
			bs.logChan <- fmt.Sprintf("✅ Thread %d (Account %d '%s'): Successful purchase! OrderID: %s, but transaction NOT sent",
				worker.workerID, accountNum, worker.account.Name, resp.OrderID)
		} else {
			// Regular successful request without TON
			bs.logChan <- fmt.Sprintf("✅ Thread %d (Account %d '%s'): Successful request!", worker.workerID, accountNum, worker.account.Name)
		}
	}
}

// Stop stops the purchase process
func (bs *BuyerService) Stop() {
	bs.mu.Lock()
	defer bs.mu.Unlock()

	if !bs.isRunning {
		return
	}

	if bs.cancel != nil {
		bs.cancel()
	}

	// Stop all snipe monitors
	for _, monitor := range bs.snipeMonitors {
		monitor.Stop()
	}
	bs.snipeMonitors = nil

	// Close transaction log file
	if bs.transactionLog != nil {
		bs.transactionLog.Close()
		bs.transactionLog = nil
	}

	// Reset active accounts tracking
	bs.activeAccountsMu.Lock()
	bs.activeAccounts = make(map[string]bool)
	bs.totalAccounts = 0
	bs.activeAccountsMu.Unlock()

	bs.isRunning = false
	bs.isStopping = false // Reset stopping flag
	bs.logChan <- "🛑 Stopping sticker purchase..."
}

// IsRunning returns the service status
func (bs *BuyerService) IsRunning() bool {
	bs.mu.RLock()
	defer bs.mu.RUnlock()
	return bs.isRunning
}

// GetStatistics returns current statistics
func (bs *BuyerService) GetStatistics() *types.Statistics {
	bs.mu.RLock()
	defer bs.mu.RUnlock()

	// Create copy of statistics
	stats := *bs.statistics
	if bs.isRunning {
		stats.Duration = time.Since(stats.StartTime)
		if stats.Duration.Seconds() > 0 {
			stats.RequestsPerSec = float64(stats.TotalRequests) / stats.Duration.Seconds()
		}
	}
	return &stats
}

// GetLogChannel returns log channel
func (bs *BuyerService) GetLogChannel() <-chan string {
	return bs.logChan
}

// updateStatistics updates statistics every second
func (bs *BuyerService) updateStatistics(ctx context.Context) {
	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			stats := bs.GetStatistics()
			activeCount, totalAccounts := bs.getActiveAccountsCount()
			bs.logChan <- fmt.Sprintf("📈 Total: %d | Successful: %d | Failed: %d | InvalidTokens: %d | TON sent: %d | RPS: %.1f | Active accounts: %d/%d | Time: %s",
				stats.TotalRequests,
				stats.SuccessRequests,
				stats.FailedRequests,
				stats.InvalidTokens,
				stats.SentTransactions,
				stats.RequestsPerSec,
				activeCount,
				totalAccounts,
				stats.Duration.Truncate(time.Second),
			)
		}
	}
}

// logTransaction logs transaction information to file
func (bs *BuyerService) logTransaction(txLog *types.TransactionLog) {
	if bs.transactionLog == nil {
		return
	}

	// Convert to JSON
	data, err := json.Marshal(txLog)
	if err != nil {
		bs.logChan <- fmt.Sprintf("❌ Transaction log error: %v", err)
		return
	}

	// Log to file
	_, err = bs.transactionLog.WriteString(string(data) + "\n")
	if err != nil {
		bs.logChan <- fmt.Sprintf("❌ Transaction log write error: %v", err)
		return
	}

	// Immediately save to disk
	bs.transactionLog.Sync()
}

// createPurchaseCallback creates callback function for purchasing stickers
func (bs *BuyerService) createPurchaseCallback(account *config.Account) monitor.PurchaseCallback {
	return func(request monitor.PurchaseRequest) error {
		bs.logChan <- fmt.Sprintf("🚀 Snipe purchase: %s (Collection: %d, Character: %d, Price: %d)",
			request.Name, request.CollectionID, request.CharacterID, request.Price)

		return bs.performSnipePurchase(account.Name, request.CollectionID, request.CharacterID)
	}
}

// checkSnipeTransactionLimit проверяет достигнут ли лимит транзакций для снайп аккаунта
func (bs *BuyerService) checkSnipeTransactionLimit(accountName string) bool {
	// Find account in configuration
	var account *config.Account
	for _, acc := range bs.config.Accounts {
		if acc.Name == accountName {
			account = &acc
			break
		}
	}
	if account == nil {
		return true // Stop if account not found
	}

	// If max_transactions is 0, no limit
	if account.MaxTransactions <= 0 {
		return false
	}

	bs.snipeCountersMu.RLock()
	currentCount := bs.snipeTransactionCounters[accountName]
	bs.snipeCountersMu.RUnlock()

	return currentCount >= account.MaxTransactions
}

// incrementSnipeTransactionCounter увеличивает счетчик транзакций для снайп аккаунта
func (bs *BuyerService) incrementSnipeTransactionCounter(accountName string) (int, bool) {
	// Find account in configuration
	var account *config.Account
	for _, acc := range bs.config.Accounts {
		if acc.Name == accountName {
			account = &acc
			break
		}
	}
	if account == nil {
		return 0, true // Stop if account not found
	}

	bs.snipeCountersMu.Lock()
	bs.snipeTransactionCounters[accountName]++
	currentCount := bs.snipeTransactionCounters[accountName]
	bs.snipeCountersMu.Unlock()

	// Check if limit is reached
	limitReached := account.MaxTransactions > 0 && currentCount >= account.MaxTransactions

	return currentCount, limitReached
}

// performSnipePurchase executes purchase through snipe monitor
func (bs *BuyerService) performSnipePurchase(accountName string, collectionID int, characterID int) error {
	// Check if transaction limit is reached
	if bs.checkSnipeTransactionLimit(accountName) {
		bs.logChan <- fmt.Sprintf("🛑 Snipe '%s': Transaction limit reached, skipping purchase", accountName)
		return fmt.Errorf("transaction limit reached for account %s", accountName)
	}

	// Get cached token (without API check)
	bearerToken, err := bs.tokenManager.GetValidToken(accountName)
	if err != nil {
		return fmt.Errorf("token retrieval error: %v", err)
	}

	// Find account in configuration
	var account *config.Account
	for _, acc := range bs.config.Accounts {
		if acc.Name == accountName {
			account = &acc
			break
		}
	}
	if account == nil {
		return fmt.Errorf("account %s not found", accountName)
	}

	// Execute purchase request
	resp, err := bs.makeSnipeOrderRequest(*account, bearerToken, collectionID, characterID)
	if err != nil {
		return fmt.Errorf("request error: %v", err)
	}

	// Check response status
	if resp.StatusCode == 401 || resp.StatusCode == 403 {
		// Token expired, try to refresh and retry request
		bs.logChan <- fmt.Sprintf("🔄 [%s] Token expired at snipe (status %d), refreshing...", accountName, resp.StatusCode)

		newToken, err := bs.tokenManager.RefreshTokenOnError(accountName, resp.StatusCode)
		if err != nil {
			return fmt.Errorf("token refresh error: %v", err)
		}

		// Retry request with new token
		resp2, err := bs.makeSnipeOrderRequest(*account, newToken, collectionID, characterID)
		if err != nil {
			return fmt.Errorf("retry request error: %v", err)
		}
		resp = resp2 // Use new response
	}

	// Log server response
	bs.logChan <- fmt.Sprintf("📡 Snipe '%s': Status %d", account.Name, resp.StatusCode)
	bs.logChan <- fmt.Sprintf("📄 Snipe '%s': Response - %s", account.Name, resp.Body)

	if resp.IsTokenError {
		bs.mu.Lock()
		bs.statistics.FailedRequests++
		bs.statistics.InvalidTokens++
		bs.mu.Unlock()

		bs.logChan <- fmt.Sprintf("🔑 Snipe '%s': Invalid authorization token! Refresh attempt...", account.Name)

		// Try to refresh token
		newToken, err := bs.tokenManager.RefreshTokenOnError(account.Name, resp.StatusCode)
		if err != nil {
			bs.logChan <- fmt.Sprintf("❌ Snipe '%s': Token refresh error: %v", account.Name, err)
			return nil
		}

		bs.logChan <- fmt.Sprintf("✅ Snipe '%s': Token refreshed successfully, retrying request...", account.Name)

		// Retry request with new token
		resp2, err := bs.makeSnipeOrderRequest(*account, newToken, collectionID, characterID)
		if err != nil {
			bs.logChan <- fmt.Sprintf("❌ Snipe '%s': Retry request error with new token: %v", account.Name, err)
			return nil
		}

		resp = resp2 // Use new response
		bs.logChan <- fmt.Sprintf("🔄 Snipe '%s': Retry request completed", account.Name)
	}

	if !resp.Success {
		bs.mu.Lock()
		bs.statistics.FailedRequests++
		bs.mu.Unlock()

		bs.logChan <- fmt.Sprintf("⚠️ Snipe '%s': Unsuccessful request (status %d)", account.Name, resp.StatusCode)
		return nil
	}

	// Successful request
	bs.mu.Lock()
	bs.statistics.SuccessRequests++
	bs.mu.Unlock()

	// Process transaction if it was sent
	if resp.TransactionSent && resp.TransactionResult != nil {
		// Update global statistics
		bs.mu.Lock()
		bs.statistics.SentTransactions++
		bs.mu.Unlock()

		// Increment snipe transaction counter
		currentCount, limitReached := bs.incrementSnipeTransactionCounter(account.Name)

		// Log transaction information
		txResult := resp.TransactionResult
		bs.logChan <- fmt.Sprintf("💰 Snipe '%s': Transaction sent!", account.Name)
		bs.logChan <- fmt.Sprintf("   📤 From address: %s", txResult.FromAddress)
		bs.logChan <- fmt.Sprintf("   📥 To address: %s", txResult.ToAddress)
		bs.logChan <- fmt.Sprintf("   💰 Amount: %.9f TON", float64(txResult.Amount)/1000000000)
		bs.logChan <- fmt.Sprintf("   🔗 Order ID: %s", resp.OrderID)
		bs.logChan <- fmt.Sprintf("   🆔 Transaction ID: %s", txResult.TransactionID)
		bs.logChan <- fmt.Sprintf("   📊 Snipe transaction count: %d/%d", currentCount, account.MaxTransactions)

		// Check if limit is reached
		if limitReached {
			bs.logChan <- fmt.Sprintf("🛑 Snipe '%s': Transaction limit reached (%d/%d) - stopping snipe monitor",
				account.Name, currentCount, account.MaxTransactions)

			// Find and stop the snipe monitor for this account
			for _, monitor := range bs.snipeMonitors {
				if monitor.GetAccountName() == account.Name {
					monitor.Stop()
					break
				}
			}

			// Mark account as inactive in the service
			bs.setAccountInactive(account.Name)
		}

		// Log transaction to file
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

// makeOrderRequest executes HTTP request for purchasing
func (bs *BuyerService) makeOrderRequest(account config.Account, bearerToken string) (*client.BuyStickersResponse, error) {
	bs.mu.Lock()
	bs.statistics.TotalRequests++
	bs.mu.Unlock()

	// Create HTTP client with account-specific proxy settings
	httpClient, err := client.NewForAccount(account.UseProxy, account.ProxyURL)
	if err != nil {
		return nil, fmt.Errorf("failed to create HTTP client for account %s: %v", account.Name, err)
	}

	// Check if seed phrase exists for sending transactions
	if account.SeedPhrase != "" {
		// Use new method with TON transaction sending and proxy support
		return httpClient.BuyStickersAndPayWithProxy(
			bearerToken,
			account.Collection,
			account.Character,
			account.Currency,
			account.Count,
			account.SeedPhrase,
			bs.config.TestMode,
			bs.config.TestAddress,
			account.UseProxy,
			account.ProxyURL,
		)
	} else {
		// Use regular method without sending transactions
		return httpClient.BuyStickers(
			bearerToken,
			account.Collection,
			account.Character,
			account.Currency,
			account.Count,
		)
	}
}

// makeSnipeOrderRequest executes HTTP request for purchasing through snipe monitor
func (bs *BuyerService) makeSnipeOrderRequest(account config.Account, bearerToken string, collectionID int, characterID int) (*client.BuyStickersResponse, error) {
	bs.mu.Lock()
	bs.statistics.TotalRequests++
	bs.mu.Unlock()

	// Create HTTP client with account-specific proxy settings
	httpClient, err := client.NewForAccount(account.UseProxy, account.ProxyURL)
	if err != nil {
		return nil, fmt.Errorf("failed to create HTTP client for account %s: %v", account.Name, err)
	}

	// Check if seed phrase exists for sending transactions
	if account.SeedPhrase != "" {
		// Use new method with TON transaction sending and proxy support
		return httpClient.BuyStickersAndPayWithProxy(
			bearerToken,
			collectionID,
			characterID,
			account.Currency,
			account.Count,
			account.SeedPhrase,
			bs.config.TestMode,
			bs.config.TestAddress,
			account.UseProxy,
			account.ProxyURL,
		)
	} else {
		// Use regular method without sending transactions
		return httpClient.BuyStickers(
			bearerToken,
			collectionID,
			characterID,
			account.Currency,
			account.Count,
		)
	}
}

// createAccountWorker creates AccountWorker with proxy support
func createAccountWorker(account config.Account, testMode bool, testAddr string, workerID int) (*AccountWorker, error) {
	// Create HTTP client with account-specific proxy settings
	httpClient, err := client.NewForAccount(account.UseProxy, account.ProxyURL)
	if err != nil {
		return nil, fmt.Errorf("failed to create HTTP client for account %s: %v", account.Name, err)
	}

	return &AccountWorker{
		client:           httpClient,
		account:          account,
		testMode:         testMode,
		testAddr:         testAddr,
		workerID:         workerID,
		transactionCount: 0,
		isActive:         true,
	}, nil
}

// setAccountInactive помечает аккаунт как неактивный и проверяет нужно ли остановить сервис
func (bs *BuyerService) setAccountInactive(accountName string) {
	bs.activeAccountsMu.Lock()
	defer bs.activeAccountsMu.Unlock()

	if bs.activeAccounts[accountName] {
		bs.activeAccounts[accountName] = false
		bs.logChan <- fmt.Sprintf("🛑 Account '%s' stopped due to transaction limit", accountName)

		// Check if all accounts are inactive
		activeCount := 0
		for _, isActive := range bs.activeAccounts {
			if isActive {
				activeCount++
			}
		}

		bs.logChan <- fmt.Sprintf("📊 Active accounts: %d/%d", activeCount, bs.totalAccounts)

		if activeCount == 0 {
			bs.logChan <- "🏁 All accounts reached transaction limits - stopping service"

			// Set stopping flag first to prevent new operations
			bs.mu.Lock()
			bs.isStopping = true
			bs.mu.Unlock()

			// Give time for current transactions to complete
			go func() {
				time.Sleep(3 * time.Second) // Wait for current operations to finish

				// Stop the service
				bs.mu.Lock()
				bs.isRunning = false
				bs.mu.Unlock()

				if bs.cancel != nil {
					bs.cancel() // Stop all goroutines
				}
			}()
		}
	}
}

// getActiveAccountsCount возвращает количество активных аккаунтов
func (bs *BuyerService) getActiveAccountsCount() (int, int) {
	bs.activeAccountsMu.RLock()
	defer bs.activeAccountsMu.RUnlock()

	activeCount := 0
	for _, isActive := range bs.activeAccounts {
		if isActive {
			activeCount++
		}
	}

	return activeCount, bs.totalAccounts
}
