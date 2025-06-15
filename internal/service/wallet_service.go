package service

import (
	"context"
	"fmt"
	"log"
	"math/big"
	"strings"

	"stickersbot/internal/client"
	"stickersbot/internal/config"
)

// WalletInfo contains wallet information and balance
type WalletInfo struct {
	AccountName string  `json:"account_name"`
	Address     string  `json:"address"`
	Balance     float64 `json:"balance"`
	Currency    string  `json:"currency"`
	Error       string  `json:"error,omitempty"`
}

// WalletService manages wallet operations
type WalletService struct {
	config *config.Config
}

// NewWalletService creates a new wallet service
func NewWalletService(cfg *config.Config) *WalletService {
	return &WalletService{
		config: cfg,
	}
}

// GetAllBalances gets balances for all accounts
func (w *WalletService) GetAllBalances(ctx context.Context) []WalletInfo {
	var wallets []WalletInfo

	for _, account := range w.config.Accounts {
		wallet := w.getAccountBalance(ctx, account)
		wallets = append(wallets, wallet)
	}

	return wallets
}

// getAccountBalance gets balance for a specific account
func (w *WalletService) getAccountBalance(ctx context.Context, account config.Account) WalletInfo {
	wallet := WalletInfo{
		AccountName: account.Name,
		Currency:    account.Currency,
	}

	// Check if seed phrase is provided
	if account.SeedPhrase == "" {
		wallet.Error = "Seed phrase not specified"
		return wallet
	}

	// Validate seed phrase
	words := strings.Fields(account.SeedPhrase)
	if len(words) != 12 && len(words) != 24 {
		wallet.Error = "Invalid seed phrase format (must be 12 or 24 words)"
		return wallet
	}

	// Create TON client from seed phrase
	tonClient, err := client.NewTONClient(account.SeedPhrase)
	if err != nil {
		wallet.Error = fmt.Sprintf("Error creating TON client: %v", err)
		return wallet
	}

	// Get wallet address
	address := tonClient.GetAddress()
	wallet.Address = address.String()

	// Get balance
	balanceNano, err := tonClient.GetBalance(ctx)
	if err != nil {
		wallet.Error = fmt.Sprintf("Error getting balance: %v", err)
		return wallet
	}

	// Convert from nanotons to TON
	balanceTON := new(big.Float).SetInt(balanceNano)
	balanceTON.Quo(balanceTON, big.NewFloat(1000000000)) // 1 TON = 1e9 nanotons
	balance, _ := balanceTON.Float64()

	wallet.Balance = balance
	log.Printf("💰 Balance for %s (%s): %.4f %s",
		account.Name, maskAddress(address.String()), balance, account.Currency)

	return wallet
}

// maskAddress masks wallet address for display
func maskAddress(address string) string {
	if len(address) < 8 {
		return strings.Repeat("*", len(address))
	}
	return address[:4] + strings.Repeat("*", len(address)-8) + address[len(address)-4:]
}
