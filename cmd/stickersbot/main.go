package main

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"stickersbot/internal/config"
	"stickersbot/internal/service"
	"stickersbot/internal/version"
)

// CLI represents the command line interface
type CLI struct {
	config          *config.Config
	authIntegration *service.AuthIntegration
	buyerService    *service.BuyerService
	tokenManager    *service.TokenManager
	walletService   *service.WalletService
	isRunning       bool
	stopChan        chan struct{}
}

// printHeader displays the ASCII art header with project info
func printHeader() {
	fmt.Println(`
╔══════════════════════════════════════════════════════════════════════════════╗
║                                                                              ║
║    ████████╗███████╗██╗     ███████╗ ██████╗ ██████╗  █████╗ ███╗   ███╗    ║
║    ╚══██╔══╝██╔════╝██║     ██╔════╝██╔════╝ ██╔══██╗██╔══██╗████╗ ████║    ║
║       ██║   █████╗  ██║     █████╗  ██║  ███╗██████╔╝███████║██╔████╔██║    ║
║       ██║   ██╔══╝  ██║     ██╔══╝  ██║   ██║██╔══██╗██╔══██║██║╚██╔╝██║    ║
║       ██║   ███████╗███████╗███████╗╚██████╔╝██║  ██║██║  ██║██║ ╚═╝ ██║    ║
║       ╚═╝   ╚══════╝╚══════╝╚══════╝ ╚═════╝ ╚═╝  ╚═╝╚═╝  ╚═╝╚═╝     ╚═╝    ║
║                                                                              ║
║                      █████╗ ██╗   ██╗████████╗ ██████╗                      ║
║                     ██╔══██╗██║   ██║╚══██╔══╝██╔═══██╗                     ║
║                     ███████║██║   ██║   ██║   ██║   ██║                     ║
║                     ██╔══██║██║   ██║   ██║   ██║   ██║                     ║
║                     ██║  ██║╚██████╔╝   ██║   ╚██████╔╝                     ║
║                     ╚═╝  ╚═╝ ╚═════╝    ╚═╝    ╚═════╝                      ║
║                                                                              ║
║                          ██████╗ ██╗   ██╗██╗   ██╗                         ║
║                          ██╔══██╗██║   ██║╚██╗ ██╔╝                         ║
║                          ██████╔╝██║   ██║ ╚████╔╝                          ║
║                          ██╔══██╗██║   ██║  ╚██╔╝                           ║
║                          ██████╔╝╚██████╔╝   ██║                            ║
║                          ╚═════╝  ╚═════╝    ╚═╝                            ║
║                                                                              ║
║                                                                              ║
║  ⚓ Developed by: DUO ON DECK Team                                           ║
║  🚀 Project: Telegram Auto Buy                                              ║
║  📧 Support: @black_beard68                                                 ║
║  📢 Channel: @two_on_deck                                                   ║
║  🌊 "Two minds, one mission - sailing the crypto seas!"                     ║
║                                                                              ║
╚══════════════════════════════════════════════════════════════════════════════╝
`)
}

func main() {
	// Display header
	printHeader()

	// Initialize CLI
	cli := &CLI{
		stopChan: make(chan struct{}),
	}

	// Load and validate configuration
	if err := cli.initializeConfig(); err != nil {
		cli.handleError("Configuration loading error", err)
		return
	}

	// Perform license check
	if err := cli.checkLicense(); err != nil {
		cli.handleError("License check error", err)
		return
	}

	// Initialize services
	if err := cli.initializeServices(); err != nil {
		cli.handleError("Services initialization error", err)
		return
	}

	// Start CLI menu
	cli.runMainMenu()
}

// initializeConfig loads and validates configuration
func (c *CLI) initializeConfig() error {
	cfgPath := "./config.json"
	cfg, err := config.Load(cfgPath)
	if err != nil {
		return fmt.Errorf("configuration loading (%s): %w", cfgPath, err)
	}

	fmt.Printf("📋 Configuration loaded: %s\n", cfgPath)
	c.config = cfg

	// Validate configuration
	if err := c.validateConfig(); err != nil {
		return fmt.Errorf("configuration validation: %w", err)
	}

	return nil
}

// validateConfig performs comprehensive configuration validation
func (c *CLI) validateConfig() error {
	var errors []string

	// Check basic configuration validity
	if !c.config.IsValid() {
		errors = append(errors, "Basic configuration is invalid")
	}

	// Check if there are accounts
	if len(c.config.Accounts) == 0 {
		errors = append(errors, "No accounts configured")
	}

	// Check each account
	for i, account := range c.config.Accounts {
		accountErrors := c.validateAccount(i+1, account)
		errors = append(errors, accountErrors...)
	}

	// Check Telegram API settings if any account needs Telegram auth
	needsTelegramAuth := false
	for _, account := range c.config.Accounts {
		if account.PhoneNumber != "" && account.AuthToken == "" {
			needsTelegramAuth = true
			break
		}
	}

	if needsTelegramAuth {
		if c.config.APIId == 0 {
			errors = append(errors, "api_id not specified for Telegram authorization")
		}
		if c.config.APIHash == "" {
			errors = append(errors, "api_hash not specified for Telegram authorization")
		}
	}

	if len(errors) > 0 {
		fmt.Println("❌ Configuration errors found:")
		for _, err := range errors {
			fmt.Printf("   • %s\n", err)
		}
		return fmt.Errorf("configuration contains %d errors", len(errors))
	}

	fmt.Println("✅ Configuration is valid")
	return nil
}

// validateAccount validates individual account configuration
func (c *CLI) validateAccount(num int, account config.Account) []string {
	var errors []string
	prefix := fmt.Sprintf("Account %d (%s)", num, account.Name)

	// Check account name
	if account.Name == "" {
		errors = append(errors, prefix+": account name not specified")
	}

	// Check authentication method
	hasPhoneAuth := account.PhoneNumber != ""
	hasTokenAuth := account.AuthToken != ""

	if !hasPhoneAuth && !hasTokenAuth {
		errors = append(errors, prefix+": neither phone_number nor auth_token specified")
	}

	// If phone auth is used, validate phone number
	if hasPhoneAuth && !strings.HasPrefix(account.PhoneNumber, "+") {
		errors = append(errors, prefix+": phone number must start with '+'")
	}

	// Check seed phrase
	if account.SeedPhrase == "" {
		errors = append(errors, prefix+": seed_phrase not specified")
	} else {
		words := strings.Fields(account.SeedPhrase)
		if len(words) != 12 && len(words) != 24 {
			errors = append(errors, prefix+": seed_phrase must contain 12 or 24 words")
		}
	}

	// Check threads
	if account.Threads <= 0 {
		errors = append(errors, prefix+": threads count must be greater than 0")
	}
	if account.Threads > 10 {
		errors = append(errors, prefix+": recommended no more than 10 threads")
	}

	// Check snipe monitor vs direct purchase mode
	if account.SnipeMonitor != nil && account.SnipeMonitor.Enabled {
		// Snipe monitor mode - collection/character are ignored
		if account.SnipeMonitor.SupplyRange == nil && account.SnipeMonitor.PriceRange == nil && len(account.SnipeMonitor.WordFilter) == 0 {
			errors = append(errors, prefix+": in snipe monitor mode at least one filter must be specified")
		}
	} else {
		// Direct purchase mode - collection/character are required
		if account.Collection <= 0 {
			errors = append(errors, prefix+": in direct purchase mode collection > 0 must be specified")
		}
		if account.Character <= 0 {
			errors = append(errors, prefix+": in direct purchase mode character > 0 must be specified")
		}
	}

	// Check currency
	if account.Currency != "TON" {
		errors = append(errors, prefix+": only TON currency is supported")
	}

	// Check count
	if account.Count <= 0 {
		errors = append(errors, prefix+": purchase count must be greater than 0")
	}

	return errors
}

// checkLicense performs license validation
func (c *CLI) checkLicense() error {
	if version.Production {
		fmt.Println("🔐 Checking license...")
		if c.config.LicenseKey == "" {
			return fmt.Errorf("license_key not specified in config.json")
		}

		err := authenticate(c.config.LicenseKey)
		if err != nil {
			return fmt.Errorf("license authentication: %w", err)
		}

		fmt.Println("✅ License authenticated successfully")
		startVerifier(c.config.LicenseKey)
	} else {
		fmt.Println("🧪 Running in development mode (license check disabled)")
		if c.config.LicenseKey == "" {
			fmt.Println("💡 Tip: Add license_key to config.json for production mode")
		}
	}

	return nil
}

// initializeServices initializes all required services
func (c *CLI) initializeServices() error {
	// Create authorization service
	c.authIntegration = service.NewAuthIntegration(c.config)

	// Validate Telegram authorization settings
	if errors := c.authIntegration.ValidateAccounts(); len(errors) > 0 {
		fmt.Println("❌ Telegram authorization settings errors:")
		for _, err := range errors {
			fmt.Printf("   • %v\n", err)
		}
		return fmt.Errorf("telegram authorization settings errors found")
	}

	// Create sessions folder if it doesn't exist
	if err := os.MkdirAll("sessions", 0755); err != nil {
		return fmt.Errorf("creating sessions folder: %w", err)
	}

	// Create token manager
	c.tokenManager = service.NewTokenManager(c.config)

	// Create buyer service
	c.buyerService = service.NewBuyerService(c.config)

	// Create wallet service
	c.walletService = service.NewWalletService(c.config)

	fmt.Println("✅ Services initialized")
	return nil
}

// handleError handles errors gracefully without immediate exit
func (c *CLI) handleError(context string, err error) {
	fmt.Printf("❌ %s: %v\n", context, err)
	fmt.Println("\n📋 Recommendations for fixing:")
	fmt.Println("   1. Check config.json file")
	fmt.Println("   2. Make sure all required fields are filled")
	fmt.Println("   3. Check phone numbers format (must start with '+')")
	fmt.Println("   4. Make sure seed_phrase contains 12 or 24 words")
	fmt.Println("   5. For Telegram authorization specify api_id and api_hash")

	fmt.Print("\nPress Enter to exit...")
	bufio.NewReader(os.Stdin).ReadLine()
}

// runMainMenu runs the main CLI menu
func (c *CLI) runMainMenu() {
	reader := bufio.NewReader(os.Stdin)

	for {
		c.printMainMenu()

		fmt.Print("Select menu option (1-4): ")
		input, _ := reader.ReadString('\n')
		choice := strings.TrimSpace(input)

		switch choice {
		case "1":
			c.handleStartTask()
		case "2":
			c.handleStopTask()
		case "3":
			c.handleShowBalances()
		case "4":
			fmt.Println("👋 Goodbye!")
			return
		default:
			fmt.Println("❌ Invalid choice. Please try again.")
		}

		fmt.Println() // Add spacing
	}
}

// printMainMenu displays the main menu
func (c *CLI) printMainMenu() {
	fmt.Println("\n" + strings.Repeat("=", 60))
	fmt.Println("                    MAIN MENU")
	fmt.Println(strings.Repeat("=", 60))

	status := "⭕ Stopped"
	if c.isRunning {
		status = "🟢 Running"
	}

	fmt.Printf("Status: %s\n", status)
	fmt.Println(strings.Repeat("-", 60))
	fmt.Println("1. 🚀 Start task (purchase/monitoring). For stop input 2 and Enter")
	fmt.Println("2. 🛑 Stop task(in running task mode)")
	fmt.Println("3. 💰 Show wallet balances")
	fmt.Println("4. 🚪 Exit")
	fmt.Println(strings.Repeat("=", 60))
}

// handleStartTask handles task start
func (c *CLI) handleStartTask() {
	if c.isRunning {
		fmt.Println("⚠️  Task is already running! Stop current task first.")
		return
	}

	fmt.Println("🔄 Preparing to start...")

	// Perform Telegram authorization for accounts that need it
	ctx := context.Background()
	if err := c.authIntegration.AuthorizeAccounts(ctx); err != nil {
		fmt.Printf("❌ Authorization error: %v\n", err)
		fmt.Print("Press Enter to continue...")
		bufio.NewReader(os.Stdin).ReadLine()
		return
	}

	// Start service
	if err := c.buyerService.Start(); err != nil {
		fmt.Printf("❌ Service startup error: %v\n", err)
		fmt.Print("Press Enter to continue...")
		bufio.NewReader(os.Stdin).ReadLine()
		return
	}

	c.isRunning = true
	fmt.Println("🚀 Task started!")
	fmt.Println("💡 Press '2' in main menu to stop")

	// Start log monitoring in background
	go c.monitorLogs()
	go c.monitorStats()
}

// handleStopTask handles task stop
func (c *CLI) handleStopTask() {
	if !c.isRunning {
		fmt.Println("⚠️  Task is not running.")
		return
	}

	fmt.Println("🛑 Stopping task...")
	c.buyerService.Stop()
	c.isRunning = false

	// Give workers time to finish gracefully
	time.Sleep(2 * time.Second)

	stats := c.buyerService.GetStatistics()
	fmt.Printf("✅ Task stopped. Statistics: Total: %d, Success: %d, Errors: %d, TON sent: %d\n",
		stats.TotalRequests, stats.SuccessRequests, stats.FailedRequests, stats.SentTransactions)
}

// handleShowBalances shows wallet balances for all accounts
func (c *CLI) handleShowBalances() {
	fmt.Println("💰 Getting wallet balances...")
	fmt.Println(strings.Repeat("-", 80))

	ctx := context.Background()
	wallets := c.walletService.GetAllBalances(ctx)

	for i, wallet := range wallets {
		fmt.Printf("Account %d: %s\n", i+1, wallet.AccountName)

		if wallet.Error != "" {
			fmt.Printf("   ❌ Error: %s\n", wallet.Error)
		} else {
			fmt.Printf("   📱 Phone: %s\n", maskPhoneNumber(c.config.Accounts[i].PhoneNumber))
			fmt.Printf("   💼 Address: %s\n", wallet.Address)
			fmt.Printf("   💰 Balance: %.4f %s\n", wallet.Balance, wallet.Currency)
		}
		fmt.Println()
	}

	fmt.Print("Press Enter to continue...")
	bufio.NewReader(os.Stdin).ReadLine()
}

// monitorLogs monitors and displays logs
func (c *CLI) monitorLogs() {
	for c.isRunning {
		select {
		case log := <-c.buyerService.GetLogChannel():
			fmt.Printf("📝 %s\n", log)
		case <-c.stopChan:
			return
		}
	}
}

// monitorStats monitors and displays statistics
func (c *CLI) monitorStats() {
	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()

	for c.isRunning {
		select {
		case <-ticker.C:
			if c.isRunning {
				stats := c.buyerService.GetStatistics()
				fmt.Printf("📈 Stats: Total: %d | Success: %d | Errors: %d | TON: %d | RPS: %.1f | Time: %s\n",
					stats.TotalRequests,
					stats.SuccessRequests,
					stats.FailedRequests,
					stats.SentTransactions,
					stats.RequestsPerSec,
					stats.Duration.Truncate(time.Second),
				)
			}
		case <-c.stopChan:
			return
		}
	}
}

// maskPhoneNumber masks phone number for display
func maskPhoneNumber(phone string) string {
	if len(phone) < 4 {
		return strings.Repeat("*", len(phone))
	}
	return phone[:3] + strings.Repeat("*", len(phone)-6) + phone[len(phone)-3:]
}

// maskSeedPhrase masks seed phrase for display
func maskSeedPhrase(seed string) string {
	words := strings.Fields(seed)
	if len(words) < 3 {
		return strings.Repeat("*", len(seed))
	}
	return words[0] + " " + strings.Repeat("*", 20) + " " + words[len(words)-1]
}

// findConfigPath returns the path to the configuration file
func findConfigPath() string {
	return "./config.json"
}
