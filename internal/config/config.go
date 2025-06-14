package config

import (
	"encoding/json"
	"os"
)

// Account structure for individual account
type Account struct {
	Name      string `json:"name"`
	AuthToken string `json:"auth_token"`

	// Telegram authentication (only phone number is unique for each account)
	PhoneNumber string `json:"phone_number,omitempty"` // Phone number for authentication
	SessionFile string `json:"session_file,omitempty"` // Path to session file (optional)

	SeedPhrase      string `json:"seed_phrase"`
	Threads         int    `json:"threads"`
	Collection      int    `json:"collection"`
	Character       int    `json:"character"`
	Currency        string `json:"currency"`
	Count           int    `json:"count"`
	MaxTransactions int    `json:"max_transactions"` // Maximum number of successful transactions

	// Snipe monitor settings
	SnipeMonitor *SnipeMonitorConfig `json:"snipe_monitor,omitempty"`
}

// SnipeMonitorConfig snipe monitor settings
type SnipeMonitorConfig struct {
	Enabled     bool     `json:"enabled"`                // Whether snipe monitor is enabled
	SupplyRange *Range   `json:"supply_range,omitempty"` // Supply range
	PriceRange  *Range   `json:"price_range,omitempty"`  // Price range (in nanotons)
	WordFilter  []string `json:"word_filter,omitempty"`  // Word filter for collection name
}

// Range structure for specifying range
type Range struct {
	Min int `json:"min"` // Minimum value
	Max int `json:"max"` // Maximum value
}

// Config application configuration structure
type Config struct {
	// API settings
	APIBaseURL string `json:"api_base_url"`
	APIKey     string `json:"api_key"`

	// Interface settings
	Theme    string `json:"theme"`
	Language string `json:"language"`

	// Network settings
	Timeout    int    `json:"timeout"`
	RetryCount int    `json:"retry_count"`
	UseProxy   bool   `json:"use_proxy"`
	ProxyURL   string `json:"proxy_url"`

	// Test settings (common for all accounts)
	TestMode    bool   `json:"test_mode"`
	TestAddress string `json:"test_address"`

	// Common Telegram settings (for all accounts)
	APIId       int    `json:"api_id"`        // API ID from my.telegram.org
	APIHash     string `json:"api_hash"`      // API Hash from my.telegram.org
	BotUsername string `json:"bot_username"`  // Bot username for token retrieval
	WebAppURL   string `json:"web_app_url"`   // Web App URL for token retrieval
	TokenAPIURL string `json:"token_api_url"` // API URL for Bearer token retrieval

	// Accounts
	Accounts []Account `json:"accounts"`
}

// Default returns default configuration
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
				Name:            "Account 1",
				AuthToken:       "",
				SeedPhrase:      "",
				Threads:         1,
				Collection:      25,
				Character:       1,
				Currency:        "TON",
				Count:           5,
				MaxTransactions: 10, // Default 10 transactions
			},
		},
	}
}

// Load loads configuration from file
func Load(filename string) (*Config, error) {
	config := Default()

	data, err := os.ReadFile(filename)
	if err != nil {
		// If file doesn't exist, return default configuration
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

// Save saves configuration to file
func (c *Config) Save(filename string) error {
	data, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(filename, data, 0644)
}

// IsValid checks configuration validity
func (c *Config) IsValid() bool {
	if len(c.Accounts) == 0 {
		return false
	}

	//// Check each account
	//for _, account := range c.Accounts {
	//	if account.AuthToken == "" || account.Threads <= 0 {
	//		return false
	//	}
	//}

	// If test mode is enabled, check for test address presence
	if c.TestMode && c.TestAddress == "" {
		return false
	}

	return true
}
