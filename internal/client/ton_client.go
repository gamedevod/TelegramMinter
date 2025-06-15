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

// TransactionRequest запрос на транзакцию
type TransactionRequest struct {
	ToAddress   string
	Amount      int64
	Comment     string
	TestMode    bool
	TestAddress string
	ResultChan  chan *TransactionResult
}

// TransactionQueue очередь транзакций для одной seed phrase
type TransactionQueue struct {
	wallet     *wallet.Wallet
	client     *ton.APIClient
	seedPhrase string
	queue      chan *TransactionRequest
	ctx        context.Context
	cancel     context.CancelFunc
	mu         sync.Mutex // Мьютекс для синхронизации транзакций
}

// NewTransactionQueue создает новую очередь транзакций
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
		queue:      make(chan *TransactionRequest, 100), // Буфер на 100 транзакций
		ctx:        ctx,
		cancel:     cancel,
	}

	// Запускаем обработчик очереди
	go tq.processQueue()

	return tq, nil
}

// processQueue обрабатывает очередь транзакций последовательно
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

// processTransaction обрабатывает одну транзакцию с ожиданием подтверждения
func (tq *TransactionQueue) processTransaction(req *TransactionRequest) *TransactionResult {
	// Блокируем доступ к кошельку на время всей операции
	// Это гарантирует что транзакции отправляются строго последовательно
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

	// Получаем текущий seqno перед отправкой транзакции
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

	// Создаем контекст с таймаутом для транзакции
	txCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Отправляем транзакцию (НЕ ждет подтверждения)
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

	// Ждем подтверждения транзакции (изменения seqno)
	expectedSeqno := initialSeqno + 1
	confirmed := false

	// Ждем до 60 секунд подтверждения
	for i := 0; i < 60; i++ {
		time.Sleep(1 * time.Second)

		currentSeqno, err := tq.getSeqno(ctx, fromAddr)
		if err != nil {
			continue // Продолжаем ждать при ошибках
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

// getSeqno получает текущий seqno для адреса
func (tq *TransactionQueue) getSeqno(ctx context.Context, addr *address.Address) (uint32, error) {
	block, err := tq.client.CurrentMasterchainInfo(ctx)
	if err != nil {
		return 0, fmt.Errorf("CurrentMasterchainInfo: %w", err)
	}

	res, err := tq.client.RunGetMethod(ctx, block, addr, "seqno")
	if err != nil {
		return 0, fmt.Errorf("RunGetMethod seqno: %w", err)
	}

	// Используем правильный способ получения результата
	if res.MustInt(0) == nil {
		return 0, fmt.Errorf("RunGetMethod seqno дал пустой ответ")
	}

	seqno := res.MustInt(0).Uint64()
	return uint32(seqno), nil
}

// AddTransaction добавляет транзакцию в очередь и ждет результата
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

	// Добавляем в очередь
	select {
	case tq.queue <- req:
		// Ждем результата (может занять до 60 секунд на транзакцию)
		result := <-resultChan
		return result
	case <-time.After(5 * time.Second):
		// Таймаут добавления в очередь
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

// Close закрывает очередь транзакций
func (tq *TransactionQueue) Close() {
	tq.cancel()
}

// WalletManager глобальный менеджер кошельков с очередями транзакций
type WalletManager struct {
	queues map[string]*TransactionQueue
	mu     sync.RWMutex
	client *ton.APIClient
}

var globalWalletManager *WalletManager
var managerOnce sync.Once

// getWalletManager возвращает глобальный экземпляр менеджера кошельков
func getWalletManager() *WalletManager {
	managerOnce.Do(func() {
		// Connect to TON mainnet
		connection := liteclient.NewConnectionPool()

		// Add public configurations
		configUrl := "https://ton.org/global.config.json"
		err := connection.AddConnectionsFromConfigUrl(context.Background(), configUrl)
		if err != nil {
			panic(fmt.Errorf("error connecting to TON: %v", err))
		}

		// Create API client
		client := ton.NewAPIClient(connection)

		globalWalletManager = &WalletManager{
			queues: make(map[string]*TransactionQueue),
			client: client,
		}
	})
	return globalWalletManager
}

// getOrCreateQueue получает или создает очередь транзакций для seed phrase
func (wm *WalletManager) getOrCreateQueue(seedPhrase string) (*TransactionQueue, error) {
	wm.mu.RLock()
	if queue, exists := wm.queues[seedPhrase]; exists {
		wm.mu.RUnlock()
		return queue, nil
	}
	wm.mu.RUnlock()

	wm.mu.Lock()
	defer wm.mu.Unlock()

	// Double-check после получения write lock
	if queue, exists := wm.queues[seedPhrase]; exists {
		return queue, nil
	}

	// Создаем новую очередь
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
}

// NewTONClient creates a new TON client
func NewTONClient(seedPhrase string) (*TONClient, error) {
	wm := getWalletManager()

	// Получаем или создаем очередь для этой seed phrase
	queue, err := wm.getOrCreateQueue(seedPhrase)
	if err != nil {
		return nil, err
	}

	return &TONClient{
		queue:      queue,
		seedPhrase: seedPhrase,
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
	// Добавляем транзакцию в очередь и ждем результата
	// Это может занять время, так как транзакция ждет подтверждения
	result := c.queue.AddTransaction(toAddress, amount, comment, testMode, testAddress)

	if !result.Success {
		return result, fmt.Errorf("transaction failed")
	}

	return result, nil
}

// GetBalance gets wallet balance
func (c *TONClient) GetBalance(ctx context.Context) (*big.Int, error) {
	wm := getWalletManager()
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
