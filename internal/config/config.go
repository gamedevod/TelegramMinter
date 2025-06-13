package config

import (
	"encoding/json"
	"os"
)

// Config структура конфигурации приложения
type Config struct {
	// Настройки API
	APIBaseURL string `json:"api_base_url"`
	APIKey     string `json:"api_key"`
	AuthToken  string `json:"auth_token"`
	
	// Настройки интерфейса
	Theme      string `json:"theme"`
	Language   string `json:"language"`
	
	// Настройки сети
	Timeout    int  `json:"timeout"`
	RetryCount int  `json:"retry_count"`
	UseProxy   bool `json:"use_proxy"`
	ProxyURL   string `json:"proxy_url"`
	
	// Настройки покупки стикеров
	Threads    int    `json:"threads"`
	TargetURL  string `json:"target_url"`
	Collection int    `json:"collection"`
	Character  int    `json:"character"`
	Currency   string `json:"currency"`
	Count      int    `json:"count"`
}

// Default возвращает конфигурацию по умолчанию
func Default() *Config {
	return &Config{
		APIBaseURL: "https://api.example.com",
		APIKey:     "",
		AuthToken:  "",
		Theme:      "default",
		Language:   "ru",
		Timeout:    30,
		RetryCount: 3,
		UseProxy:   false,
		ProxyURL:   "",
		Threads:    1,
		TargetURL:  "https://api.stickerdom.store/api/v1/shop/buy/crypto",
		Collection: 25,
		Character:  1,
		Currency:   "TON",
		Count:      5,
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
	return c.AuthToken != "" && c.Threads > 0
} 