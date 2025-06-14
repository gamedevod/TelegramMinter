package client

import (
	"context"
	"fmt"
	"math/big"
	"strings"

	"github.com/xssnick/tonutils-go/address"
	"github.com/xssnick/tonutils-go/liteclient"
	"github.com/xssnick/tonutils-go/tlb"
	"github.com/xssnick/tonutils-go/ton"
	"github.com/xssnick/tonutils-go/ton/wallet"
)

// TONClient client for working with TON blockchain
type TONClient struct {
	client *ton.APIClient
	wallet *wallet.Wallet
}

// NewTONClient creates a new TON client
func NewTONClient(seedPhrase string) (*TONClient, error) {
	// Connect to TON mainnet
	connection := liteclient.NewConnectionPool()

	// Add public configurations
	configUrl := "https://ton.org/global.config.json"
	err := connection.AddConnectionsFromConfigUrl(context.Background(), configUrl)
	if err != nil {
		return nil, fmt.Errorf("error connecting to TON: %v", err)
	}

	// Create API client
	client := ton.NewAPIClient(connection)

	// Create wallet from seed phrase
	words := strings.Split(seedPhrase, " ")
	if len(words) != 24 {
		return nil, fmt.Errorf("incorrect number of words in seed phrase: %d (should be 24)", len(words))
	}

	// Create wallet from seed
	w, err := wallet.FromSeed(client, words, wallet.V4R2)
	if err != nil {
		return nil, fmt.Errorf("error creating wallet: %v", err)
	}

	return &TONClient{
		client: client,
		wallet: w,
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

// SendTON sends TON transaction and returns information about it
func (c *TONClient) SendTON(ctx context.Context, toAddress string, amount int64, comment string, testMode bool, testAddress string) (*TransactionResult, error) {
	// If test mode, use test address
	if testMode && testAddress != "" {
		toAddress = testAddress
	}

	// Parse recipient address
	addr, err := address.ParseAddr(toAddress)
	if err != nil {
		return nil, fmt.Errorf("error parsing address: %v", err)
	}

	// Get sender address
	fromAddr := c.wallet.WalletAddress()

	// Send transaction
	err = c.wallet.Transfer(ctx, addr, tlb.FromNanoTONU(uint64(amount)), comment)
	if err != nil {
		return &TransactionResult{
			FromAddress:   fromAddr.String(),
			ToAddress:     toAddress,
			TransactionID: "",
			Amount:        amount,
			Comment:       comment,
			Success:       false,
		}, fmt.Errorf("error sending transaction: %v", err)
	}

	// Return result with temporary ID
	result := &TransactionResult{
		FromAddress:   fromAddr.String(),
		ToAddress:     toAddress,
		TransactionID: fmt.Sprintf("tx_%d_%s", amount, comment), // Temporary ID
		Amount:        amount,
		Comment:       comment,
		Success:       true,
	}

	return result, nil
}

// GetBalance gets wallet balance
func (c *TONClient) GetBalance(ctx context.Context) (*big.Int, error) {
	block, err := c.client.CurrentMasterchainInfo(ctx)
	if err != nil {
		return nil, err
	}

	balance, err := c.wallet.GetBalance(ctx, block)
	if err != nil {
		return nil, err
	}

	return balance.NanoTON(), nil
}

// GetAddress returns wallet address
func (c *TONClient) GetAddress() *address.Address {
	return c.wallet.WalletAddress()
}
