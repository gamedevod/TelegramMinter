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

// TONClient клиент для работы с TON блокчейном
type TONClient struct {
	client *ton.APIClient
	wallet *wallet.Wallet
}

// NewTONClient создает новый TON клиент
func NewTONClient(seedPhrase string) (*TONClient, error) {
	// Подключаемся к TON mainnet
	connection := liteclient.NewConnectionPool()

	// Добавляем публичные конфигурации
	configUrl := "https://ton.org/global.config.json"
	err := connection.AddConnectionsFromConfigUrl(context.Background(), configUrl)
	if err != nil {
		return nil, fmt.Errorf("ошибка подключения к TON: %v", err)
	}

	// Создаем API клиент
	client := ton.NewAPIClient(connection)

	// Создаем кошелек из seed фразы
	words := strings.Split(seedPhrase, " ")
	if len(words) != 24 {
		return nil, fmt.Errorf("неверное количество слов в seed фразе: %d (должно быть 24)", len(words))
	}

	// Создаем кошелек из seed
	w, err := wallet.FromSeed(client, words, wallet.V4R2)
	if err != nil {
		return nil, fmt.Errorf("ошибка создания кошелька: %v", err)
	}

	return &TONClient{
		client: client,
		wallet: w,
	}, nil
}

// SendTON отправляет TON транзакцию
func (c *TONClient) SendTON(ctx context.Context, toAddress string, amount int64, comment string, testMode bool, testAddress string) error {
	// Если тестовый режим, используем тестовый адрес
	if testMode && testAddress != "" {
		toAddress = testAddress
	}

	// Парсим адрес получателя
	addr, err := address.ParseAddr(toAddress)
	if err != nil {
		return fmt.Errorf("ошибка парсинга адреса: %v", err)
	}

	// Используем простой метод Transfer
	err = c.wallet.Transfer(ctx, addr, tlb.FromNanoTONU(uint64(amount)), comment)
	if err != nil {
		return fmt.Errorf("ошибка отправки транзакции: %v", err)
	}

	return nil
}

// GetBalance получает баланс кошелька
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

// GetAddress возвращает адрес кошелька
func (c *TONClient) GetAddress() *address.Address {
	return c.wallet.WalletAddress()
}
