package config

import (
	"encoding/json"
	"os"
)

// Account structure for individual account
type Account struct {
	Name      string `json:"name"`
	AuthToken string `json:"auth_token"`

	// Telegram authentication settings (individual for each account)
	APIId             int    `json:"api_id"`                        // API ID from my.telegram.org (individual for each account)
	APIHash           string `json:"api_hash"`                      // API Hash from my.telegram.org (individual for each account)
	PhoneNumber       string `json:"phone_number,omitempty"`        // Phone number for authentication
	SessionFile       string `json:"session_file,omitempty"`        // Path to session file (optional)
	TwoFactorPassword string `json:"two_factor_password,omitempty"` // 2FA password (optional, leave empty to prompt)

	SeedPhrase      string `json:"seed_phrase"`
	Threads         int    `json:"threads"`
	Collection      int    `json:"collection"`
	Character       int    `json:"character"`
	Currency        string `json:"currency"`
	Count           int    `json:"count"`
	MaxTransactions int    `json:"max_transactions"` // Maximum number of successful transactions

	// Proxy settings (individual for each account)
	UseProxy bool   `json:"use_proxy,omitempty"` // Whether to use proxy for this account
	ProxyURL string `json:"proxy_url,omitempty"` // Proxy URL in format host:port:user:pass

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
	// License settings
	LicenseKey string `json:"license_key"`

	// Interface settings
	Theme    string `json:"theme"`
	Language string `json:"language"`

	// Network settings
	Timeout int `json:"timeout"`

	// Test settings (common for all accounts)
	TestMode    bool   `json:"test_mode"`
	TestAddress string `json:"test_address"`

	// Accounts (each account now has individual API credentials)
	Accounts []Account `json:"accounts"`
}

// Default returns default configuration
func Default() *Config {
	return &Config{
		LicenseKey:  "",
		Theme:       "default",
		Language:    "ru",
		Timeout:     30,
		TestMode:    false,
		TestAddress: "",
		Accounts: []Account{
			{
				Name:            "Account 1",
				AuthToken:       "",
				APIId:           0,  // Your API ID from my.telegram.org
				APIHash:         "", // Your API Hash from my.telegram.org
				SeedPhrase:      "",
				Threads:         1,
				Collection:      25,
				Character:       1,
				Currency:        "TON",
				Count:           5,
				MaxTransactions: 10, // Default 10 transactions
				UseProxy:        false,
				ProxyURL:        "", // Example: "proxy.example.com:1080:username:password"
			},
			{
				Name:            "Account 2 (with proxy example)",
				AuthToken:       "",
				APIId:           0,  // Your API ID from my.telegram.org
				APIHash:         "", // Your API Hash from my.telegram.org
				SeedPhrase:      "",
				Threads:         1,
				Collection:      25,
				Character:       1,
				Currency:        "TON",
				Count:           5,
				MaxTransactions: 10,
				UseProxy:        false,                                      // Set to true to enable proxy
				ProxyURL:        "proxy.example.com:1080:username:password", // Example proxy URL
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
