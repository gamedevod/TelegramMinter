package storage

import (
	"encoding/json"
	"os"
	"sync"
)

// TokenStorage обеспечивает потокобезопасное хранение токенов Bearer
// отдельно от основного конфигурационного файла.
// Токены хранятся в простом JSON-объекте вида { "Account Name": "token" }.
// Такой формат позволяет избежать конфликтов записи при работе в многопоточном режиме.

type TokenStorage struct {
	file   string
	tokens map[string]string
	mu     sync.RWMutex
}

// NewTokenStorage загружает хранилище токенов из указанного файла
// либо создаёт новое, если файл отсутствует.
func NewTokenStorage(file string) (*TokenStorage, error) {
	ts := &TokenStorage{
		file:   file,
		tokens: make(map[string]string),
	}

	data, err := os.ReadFile(file)
	if err != nil {
		if os.IsNotExist(err) {
			// Файл отсутствует – это не ошибка.
			return ts, nil
		}
		return nil, err
	}

	// Пытаемся десериализовать. В случае ошибки начинаем с пустой мапы.
	_ = json.Unmarshal(data, &ts.tokens)

	return ts, nil
}

// GetToken возвращает токен для указанного аккаунта.
func (ts *TokenStorage) GetToken(accountName string) (string, bool) {
	ts.mu.RLock()
	defer ts.mu.RUnlock()
	token, ok := ts.tokens[accountName]
	return token, ok
}

// SetToken сохраняет токен и моментально пишет изменения на диск.
func (ts *TokenStorage) SetToken(accountName, token string) error {
	ts.mu.Lock()
	defer ts.mu.Unlock()

	ts.tokens[accountName] = token
	return ts.persist()
}

// persist выполняет запись на диск. Вызывать только под мьютексом.
func (ts *TokenStorage) persist() error {
	data, err := json.MarshalIndent(ts.tokens, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(ts.file, data, 0o644)
}
