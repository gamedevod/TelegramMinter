package client

import (
	"context"
	"fmt"
	"math/big"
	"strings"
	"sync"
	"time"

	"github.com/xssnick/tonutils-go/address"
	"github.com/xssnick/tonutils-go/liteclient"
	"github.com/xssnick/tonutils-go/tlb"
	"github.com/xssnick/tonutils-go/ton"
	"github.com/xssnick/tonutils-go/ton/wallet"
)

// TransactionRequest transaction request structure
type TransactionRequest struct {
	ToAddress   string
	Amount      int64
	Comment     string
	TestMode    bool
	TestAddress string
	ResultChan  chan *TransactionResult
}

// TransactionQueue transaction queue for one seed phrase
type TransactionQueue struct {
	wallet     *wallet.Wallet
	client     *ton.APIClient
	seedPhrase string
	queue      chan *TransactionRequest
	ctx        context.Context
	cancel     context.CancelFunc
	mu         sync.Mutex // Mutex for transaction synchronization
}

// NewTransactionQueue creates a new transaction queue
func NewTransactionQueue(seedPhrase string, client *ton.APIClient) (*TransactionQueue, error) {
	words := strings.Split(seedPhrase, " ")
	if len(words) != 24 {
		return nil, fmt.Errorf("incorrect number of words in seed phrase: %d (should be 24)", len(words))
	}

	// Create wallet from seed
	w, err := wallet.FromSeed(client, words, wallet.V4R2)
	if err != nil {
		return nil, fmt.Errorf("error creating wallet: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())

	tq := &TransactionQueue{
		wallet:     w,
		client:     client,
		seedPhrase: seedPhrase,
		queue:      make(chan *TransactionRequest, 100), // Buffer for 100 transactions
		ctx:        ctx,
		cancel:     cancel,
	}

	// Start queue processor
	go tq.processQueue()

	return tq, nil
}

// processQueue processes transaction queue sequentially
func (tq *TransactionQueue) processQueue() {
	for {
		select {
		case <-tq.ctx.Done():
			return
		case req := <-tq.queue:
			result := tq.processTransaction(req)
			req.ResultChan <- result
		}
	}
}

// processTransaction processes one transaction with confirmation waiting
func (tq *TransactionQueue) processTransaction(req *TransactionRequest) *TransactionResult {
	// Lock wallet access for the entire operation
	// This ensures transactions are sent strictly sequentially
	tq.mu.Lock()
	defer tq.mu.Unlock()

	toAddress := req.ToAddress
	if req.TestMode && req.TestAddress != "" {
		toAddress = req.TestAddress
	}

	// Parse recipient address
	addr, err := address.ParseAddr(toAddress)
	if err != nil {
		return &TransactionResult{
			FromAddress:   tq.wallet.WalletAddress().String(),
			ToAddress:     toAddress,
			TransactionID: "",
			Amount:        req.Amount,
			Comment:       req.Comment,
			Success:       false,
		}
	}

	// Get sender address
	fromAddr := tq.wallet.WalletAddress()

	// Get current seqno before sending transaction
	ctx := context.Background()

	initialSeqno, err := tq.getSeqno(ctx, fromAddr)
	if err != nil {
		return &TransactionResult{
			FromAddress:   fromAddr.String(),
			ToAddress:     toAddress,
			TransactionID: "",
			Amount:        req.Amount,
			Comment:       req.Comment,
			Success:       false,
		}
	}

	// Create context with timeout for transaction
	txCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Send transaction (does NOT wait for confirmation)
	err = tq.wallet.Transfer(txCtx, addr, tlb.FromNanoTONU(uint64(req.Amount)), req.Comment)
	if err != nil {
		return &TransactionResult{
			FromAddress:   fromAddr.String(),
			ToAddress:     toAddress,
			TransactionID: "",
			Amount:        req.Amount,
			Comment:       req.Comment,
			Success:       false,
		}
	}

	// Wait for transaction confirmation (seqno change)
	expectedSeqno := initialSeqno + 1
	confirmed := false

	// Wait up to 60 seconds for confirmation
	for i := 0; i < 60; i++ {
		time.Sleep(1 * time.Second)

		currentSeqno, err := tq.getSeqno(ctx, fromAddr)
		if err != nil {
			continue // Continue waiting on errors
		}

		if currentSeqno >= expectedSeqno {
			confirmed = true
			break
		}
	}

	if !confirmed {
		return &TransactionResult{
			FromAddress:   fromAddr.String(),
			ToAddress:     toAddress,
			TransactionID: "",
			Amount:        req.Amount,
			Comment:       req.Comment,
			Success:       false,
		}
	}

	// Return successful result
	result := &TransactionResult{
		FromAddress:   fromAddr.String(),
		ToAddress:     toAddress,
		TransactionID: fmt.Sprintf("tx_%d_%s_%s_%d", req.Amount, req.Comment, fromAddr.String(), time.Now().Unix()),
		Amount:        req.Amount,
		Comment:       req.Comment,
		Success:       true,
	}

	return result
}

// getSeqno gets current seqno for address
func (tq *TransactionQueue) getSeqno(ctx context.Context, addr *address.Address) (uint32, error) {
	block, err := tq.client.CurrentMasterchainInfo(ctx)
	if err != nil {
		return 0, fmt.Errorf("CurrentMasterchainInfo: %w", err)
	}

	res, err := tq.client.RunGetMethod(ctx, block, addr, "seqno")
	if err != nil {
		// Check if error is result of undeployed wallet
		errStr := err.Error()
		if strings.Contains(errStr, "account not found") ||
			strings.Contains(errStr, "contract not found") ||
			strings.Contains(errStr, "account is not active") ||
			strings.Contains(errStr, "exit code") {

			fmt.Printf("⚠️  Wallet not deployed, starting automatic deployment...\n")

			// Attempt automatic deployment
			deployErr := tq.deployWalletIfNeeded(ctx)
			if deployErr != nil {
				return 0, fmt.Errorf("wallet deployment error: %w", deployErr)
			}

			// Retry getting seqno after deployment
			block, err = tq.client.CurrentMasterchainInfo(ctx)
			if err != nil {
				return 0, fmt.Errorf("CurrentMasterchainInfo after deployment: %w", err)
			}

			res, err = tq.client.RunGetMethod(ctx, block, addr, "seqno")
			if err != nil {
				return 0, fmt.Errorf("RunGetMethod seqno after deployment: %w", err)
			}
		} else {
			return 0, fmt.Errorf("RunGetMethod seqno: %w", err)
		}
	}

	// Use correct way to get result
	if res.MustInt(0) == nil {
		return 0, fmt.Errorf("RunGetMethod seqno returned empty result")
	}

	seqno := res.MustInt(0).Uint64()
	return uint32(seqno), nil
}

// deployWalletIfNeeded deploys wallet if not yet deployed
func (tq *TransactionQueue) deployWalletIfNeeded(ctx context.Context) error {
	fmt.Printf("🔍 Checking wallet balance for deployment...\n")

	// Check current wallet balance
	block, err := tq.client.CurrentMasterchainInfo(ctx)
	if err != nil {
		return fmt.Errorf("CurrentMasterchainInfo: %w", err)
	}

	balance, err := tq.wallet.GetBalance(ctx, block)
	if err != nil {
		return fmt.Errorf("getting balance: %w", err)
	}

	balanceNano := balance.NanoTON()
	balanceTON := formatTON(balanceNano)

	fmt.Printf("💰 Wallet balance: %s TON\n", balanceTON)

	// Check if there are enough funds for deployment (minimum 0.05 TON required)
	minDeployAmount := big.NewInt(50000000) // 0.05 TON in nanotokens
	if balanceNano.Cmp(minDeployAmount) < 0 {
		return fmt.Errorf("insufficient funds for wallet deployment. Need minimum 0.05 TON, available: %s TON", balanceTON)
	}

	fmt.Printf("🚀 Starting wallet deployment...\n")

	// Deploy wallet by sending minimal transaction to self
	deployAmount := big.NewInt(1000000) // 0.001 TON in nanotokens
	selfAddr := tq.wallet.WalletAddress()

	deployCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	fmt.Printf("📤 Sending deployment transaction (0.001 TON)...\n")

	err = tq.wallet.Transfer(deployCtx, selfAddr, tlb.FromNanoTONU(deployAmount.Uint64()), "🚀 Wallet deployment")
	if err != nil {
		return fmt.Errorf("deployment transaction send error: %w", err)
	}

	fmt.Printf("✅ Deployment transaction sent\n")
	fmt.Printf("⏳ Waiting for deployment confirmation (up to 60 seconds)...\n")

	// Wait for deployment up to 60 seconds
	for i := 0; i < 60; i++ {
		time.Sleep(1 * time.Second)

		if i%10 == 0 && i > 0 {
			fmt.Printf("⏳ Waiting %d/60 seconds...\n", i)
		}

		// Check if wallet is deployed
		currentBlock, blockErr := tq.client.CurrentMasterchainInfo(ctx)
		if blockErr != nil {
			continue // Skip block errors
		}

		_, seqnoErr := tq.client.RunGetMethod(ctx, currentBlock, selfAddr, "seqno")
		if seqnoErr == nil {
			fmt.Printf("🎉 Wallet successfully deployed!\n")
			fmt.Printf("✅ Now transactions can be sent\n")
			return nil
		}
	}

	return fmt.Errorf("wallet deployment timeout (60 seconds). Please retry the operation")
}

// formatTON formats nanotokens to readable format
func formatTON(nanoTON *big.Int) string {
	ton := new(big.Float).SetInt(nanoTON)
	ton.Quo(ton, big.NewFloat(1e9))
	return ton.Text('f', 4)
}

// AddTransaction adds transaction to queue and waits for result
func (tq *TransactionQueue) AddTransaction(toAddress string, amount int64, comment string, testMode bool, testAddress string) *TransactionResult {
	resultChan := make(chan *TransactionResult, 1)

	req := &TransactionRequest{
		ToAddress:   toAddress,
		Amount:      amount,
		Comment:     comment,
		TestMode:    testMode,
		TestAddress: testAddress,
		ResultChan:  resultChan,
	}

	// Add to queue
	select {
	case tq.queue <- req:
		// Wait for result (may take up to 60 seconds per transaction)
		result := <-resultChan
		return result
	case <-time.After(5 * time.Second):
		// Queue addition timeout
		return &TransactionResult{
			FromAddress:   tq.wallet.WalletAddress().String(),
			ToAddress:     toAddress,
			TransactionID: "",
			Amount:        amount,
			Comment:       comment,
			Success:       false,
		}
	}
}

// Close closes transaction queue
func (tq *TransactionQueue) Close() {
	tq.cancel()
}

// WalletManager global wallet manager with transaction queues
type WalletManager struct {
	queues map[string]*TransactionQueue
	mu     sync.RWMutex
	client *ton.APIClient
}

// WalletManagerKey key for wallet manager instances
type WalletManagerKey struct {
	UseProxy bool
	ProxyURL string
}

var globalWalletManagers = make(map[WalletManagerKey]*WalletManager)
var managersOnce sync.Once
var managersMu sync.RWMutex

// getWalletManager returns wallet manager instance for specific proxy settings
func getWalletManager(useProxy bool, proxyURL string) *WalletManager {
	key := WalletManagerKey{UseProxy: useProxy, ProxyURL: proxyURL}

	managersMu.RLock()
	if manager, exists := globalWalletManagers[key]; exists {
		managersMu.RUnlock()
		return manager
	}
	managersMu.RUnlock()

	managersMu.Lock()
	defer managersMu.Unlock()

	// Double check after getting write lock
	if manager, exists := globalWalletManagers[key]; exists {
		return manager
	}

	// Create new manager
	manager := createWalletManager(useProxy, proxyURL)
	globalWalletManagers[key] = manager
	return manager
}

// createWalletManager creates a new wallet manager with optional proxy
func createWalletManager(useProxy bool, proxyURL string) *WalletManager {
	// Connect to TON mainnet
	connection := liteclient.NewConnectionPool()

	// TODO: Add proxy support to liteclient when available
	// For now, note that TON liteclient doesn't support proxy directly
	// This would require custom implementation or waiting for library update

	// Add public configurations
	configUrl := "https://ton.org/global.config.json"
	err := connection.AddConnectionsFromConfigUrl(context.Background(), configUrl)
	if err != nil {
		panic(fmt.Errorf("error connecting to TON: %v", err))
	}

	// Create API client
	client := ton.NewAPIClient(connection)

	return &WalletManager{
		queues: make(map[string]*TransactionQueue),
		client: client,
	}
}

// getOrCreateQueue gets or creates transaction queue for seed phrase
func (wm *WalletManager) getOrCreateQueue(seedPhrase string) (*TransactionQueue, error) {
	wm.mu.RLock()
	if queue, exists := wm.queues[seedPhrase]; exists {
		wm.mu.RUnlock()
		return queue, nil
	}
	wm.mu.RUnlock()

	wm.mu.Lock()
	defer wm.mu.Unlock()

	// Double-check after getting write lock
	if queue, exists := wm.queues[seedPhrase]; exists {
		return queue, nil
	}

	// Create new queue
	queue, err := NewTransactionQueue(seedPhrase, wm.client)
	if err != nil {
		return nil, err
	}

	wm.queues[seedPhrase] = queue
	return queue, nil
}

// TONClient client for working with TON blockchain
type TONClient struct {
	queue      *TransactionQueue
	seedPhrase string
	useProxy   bool
	proxyURL   string
}

// NewTONClient creates a new TON client without proxy
func NewTONClient(seedPhrase string) (*TONClient, error) {
	return NewTONClientWithProxy(seedPhrase, false, "")
}

// NewTONClientWithProxy creates a new TON client with proxy support
func NewTONClientWithProxy(seedPhrase string, useProxy bool, proxyURL string) (*TONClient, error) {
	wm := getWalletManager(useProxy, proxyURL)

	// Get or create queue for this seed phrase
	queue, err := wm.getOrCreateQueue(seedPhrase)
	if err != nil {
		return nil, err
	}

	return &TONClient{
		queue:      queue,
		seedPhrase: seedPhrase,
		useProxy:   useProxy,
		proxyURL:   proxyURL,
	}, nil
}

// TransactionResult transaction result structure
type TransactionResult struct {
	FromAddress   string
	ToAddress     string
	TransactionID string
	Amount        int64
	Comment       string
	Success       bool
}

// SendTON sends TON transaction through queue and returns information about it
func (c *TONClient) SendTON(ctx context.Context, toAddress string, amount int64, comment string, testMode bool, testAddress string) (*TransactionResult, error) {
	// Add transaction to queue and wait for result
	// This may take time as transaction waits for confirmation
	result := c.queue.AddTransaction(toAddress, amount, comment, testMode, testAddress)

	if !result.Success {
		return result, fmt.Errorf("transaction failed")
	}

	return result, nil
}

// GetBalance gets wallet balance
func (c *TONClient) GetBalance(ctx context.Context) (*big.Int, error) {
	wm := getWalletManager(c.useProxy, c.proxyURL)
	block, err := wm.client.CurrentMasterchainInfo(ctx)
	if err != nil {
		return nil, err
	}

	balance, err := c.queue.wallet.GetBalance(ctx, block)
	if err != nil {
		return nil, err
	}

	return balance.NanoTON(), nil
}

// GetAddress returns wallet address
func (c *TONClient) GetAddress() *address.Address {
	return c.queue.wallet.WalletAddress()
}
