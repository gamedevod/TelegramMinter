package config

import (
	"encoding/json"
	"os"
)

// Account структура для отдельного аккаунта
type Account struct {
	Name       string `json:"name"`
	AuthToken  string `json:"auth_token"`
	SeedPhrase string `json:"seed_phrase"`
	Threads    int    `json:"threads"`
	Collection int    `json:"collection"`
	Character  int    `json:"character"`
	Currency   string `json:"currency"`
	Count      int    `json:"count"`
}

// Config структура конфигурации приложения
type Config struct {
	// Настройки API
	APIBaseURL string `json:"api_base_url"`
	APIKey     string `json:"api_key"`

	// Настройки интерфейса
	Theme    string `json:"theme"`
	Language string `json:"language"`

	// Настройки сети
	Timeout    int    `json:"timeout"`
	RetryCount int    `json:"retry_count"`
	UseProxy   bool   `json:"use_proxy"`
	ProxyURL   string `json:"proxy_url"`

	// Тестовые настройки (общие для всех аккаунтов)
	TestMode    bool   `json:"test_mode"`
	TestAddress string `json:"test_address"`

	// Аккаунты
	Accounts []Account `json:"accounts"`
}

// Default возвращает конфигурацию по умолчанию
func Default() *Config {
	return &Config{
		APIBaseURL:  "https://api.example.com",
		APIKey:      "",
		Theme:       "default",
		Language:    "ru",
		Timeout:     30,
		RetryCount:  3,
		UseProxy:    false,
		ProxyURL:    "",
		TestMode:    false,
		TestAddress: "",
		Accounts: []Account{
			{
				Name:       "Account 1",
				AuthToken:  "",
				SeedPhrase: "",
				Threads:    1,
				Collection: 25,
				Character:  1,
				Currency:   "TON",
				Count:      5,
			},
		},
	}
}

// Load загружает конфигурацию из файла
func Load(filename string) (*Config, error) {
	config := Default()

	data, err := os.ReadFile(filename)
	if err != nil {
		// Если файл не существует, возвращаем конфигурацию по умолчанию
		if os.IsNotExist(err) {
			return config, nil
		}
		return nil, err
	}

	err = json.Unmarshal(data, config)
	if err != nil {
		return nil, err
	}

	return config, nil
}

// Save сохраняет конфигурацию в файл
func (c *Config) Save(filename string) error {
	data, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(filename, data, 0644)
}

// IsValid проверяет валидность конфигурации
func (c *Config) IsValid() bool {
	if len(c.Accounts) == 0 {
		return false
	}

	// Проверяем каждый аккаунт
	for _, account := range c.Accounts {
		if account.AuthToken == "" || account.Threads <= 0 {
			return false
		}
	}

	// Если включен тестовый режим, проверяем наличие тестового адреса
	if c.TestMode && c.TestAddress == "" {
		return false
	}

	return true
}
