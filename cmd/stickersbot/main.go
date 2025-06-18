package main

import (
	"bufio"
	"context"
	"fmt"
	"math/big"
	"os"
	"strconv"
	"strings"
	"time"

	"stickersbot/internal/client"
	"stickersbot/internal/config"
	"stickersbot/internal/service"
	"stickersbot/internal/storage"
)

// CLI represents the command line interface
type CLI struct {
	config          *config.Config
	authIntegration *service.AuthIntegration
	buyerService    *service.BuyerService
	tokenManager    *service.TokenManager
	walletService   *service.WalletService
	tokenStorage    *storage.TokenStorage
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

	//// Perform license check
	//if err := cli.checkLicense(); err != nil {
	//	cli.handleError("License check error", err)
	//	return
	//}

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

	// Загружаем хранилище токенов и подмешиваем токены в конфиг
	ts, err := storage.NewTokenStorage("tokens.json")
	if err != nil {
		return fmt.Errorf("loading token storage: %w", err)
	}

	// Проставляем токены в конфиг, если они есть в хранилище
	for i, account := range cfg.Accounts {
		if token, ok := ts.GetToken(account.Name); ok {
			cfg.Accounts[i].AuthToken = token
		} else if account.AuthToken != "" {
			// Миграция: переносим токен из конфига в отдельное хранилище
			if err := ts.SetToken(account.Name, account.AuthToken); err == nil {
				fmt.Printf("🔄 Migrated token for account '%s' to tokens.json\n", account.Name)
			}
		}

		// Включаем обязательное использование прокси
		cfg.Accounts[i].UseProxy = true
		cfg.Accounts[i].ProxyURL = "" // будет выбран случайный из proxies.txt
	}

	c.config = cfg
	c.tokenStorage = ts

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

	// Individual API validation is now handled in validateAccount function
	// Each account must have its own API credentials

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

	// If phone auth is used, validate phone number and API credentials
	if hasPhoneAuth {
		if !strings.HasPrefix(account.PhoneNumber, "+") {
			errors = append(errors, prefix+": phone number must start with '+'")
		}

		// Validate individual API credentials for this account
		if account.APIId == 0 {
			errors = append(errors, prefix+": api_id not specified (required for phone authentication)")
		}
		if account.APIHash == "" {
			errors = append(errors, prefix+": api_hash not specified (required for phone authentication)")
		}
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
		errors = append(errors, prefix+": threads must be greater than 0")
	}

	// Check collection and character
	if account.Collection <= 0 {
		errors = append(errors, prefix+": collection must be greater than 0")
	}

	// Check currency
	if account.Currency == "" {
		errors = append(errors, prefix+": currency not specified")
	}

	// Check count
	if account.Count <= 0 {
		errors = append(errors, prefix+": count must be greater than 0")
	}

	return errors
}

// validateProxyURL validates proxy URL format
func validateProxyURL(proxyURL string) error {
	parts := strings.Split(proxyURL, ":")
	if len(parts) != 2 && len(parts) != 4 {
		return fmt.Errorf("invalid proxy URL format, expected host:port or host:port:user:pass")
	}

	// Validate host
	if parts[0] == "" {
		return fmt.Errorf("proxy host cannot be empty")
	}

	// Validate port
	if parts[1] == "" {
		return fmt.Errorf("proxy port cannot be empty")
	}
	if _, err := strconv.Atoi(parts[1]); err != nil {
		return fmt.Errorf("proxy port must be a number")
	}

	// If auth is provided, validate user and pass
	if len(parts) == 4 {
		if parts[2] == "" {
			return fmt.Errorf("proxy username cannot be empty when authentication is provided")
		}
		if parts[3] == "" {
			return fmt.Errorf("proxy password cannot be empty when authentication is provided")
		}
	}

	return nil
}

// checkLicense performs license validation (currently disabled for development)
func (c *CLI) checkLicense() error {
	fmt.Println("🔐 Checking license...")

	// License check is currently disabled for development
	// In production, this would validate the license key
	if true { // Change to false to disable license checking
		if c.config.LicenseKey == "" {
			return fmt.Errorf("license_key not specified in config.json")
		}

		// Authenticate license key
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
	c.authIntegration = service.NewAuthIntegration(c.config, c.tokenStorage)

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
	c.tokenManager = service.NewTokenManager(c.config, c.tokenStorage)

	// Create buyer service
	c.buyerService = service.NewBuyerService(c.config, c.tokenStorage)

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

		fmt.Print("Select menu option (1-6): ")
		input, _ := reader.ReadString('\n')
		choice := strings.TrimSpace(input)

		switch choice {
		case "1":
			c.handleStartTask()
		case "2":
			c.handleStopTask()
		case "3":
			c.handleManageAccountAuthentication()
		case "4":
			c.handleShowBalances()
		case "5":
			c.handleCheckDeployWallets()
		case "6":
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
	fmt.Println("1. 🚀 Start task (purchase/monitoring)")
	fmt.Println("2. 🛑 Stop task")
	fmt.Println("3. 🔐 Manage account authentication")
	fmt.Println("4. 💰 Show wallet balances")
	fmt.Println("5. 🔧 Check/Deploy wallets")
	fmt.Println("6. 🚪 Exit")
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

	fmt.Printf("\n💡 Press Enter to return to main menu...")

	// Wait for user input
	bufio.NewReader(os.Stdin).ReadLine()
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
	for c.isRunning && c.buyerService.IsRunning() {
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

	for c.isRunning && c.buyerService.IsRunning() {
		select {
		case <-ticker.C:
			if c.isRunning && c.buyerService.IsRunning() {
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

	// Show final stats when service stops automatically
	if c.isRunning && !c.buyerService.IsRunning() {
		stats := c.buyerService.GetStatistics()
		fmt.Printf("🏁 Final Stats: Total: %d | Success: %d | Errors: %d | TON: %d | Time: %s\n",
			stats.TotalRequests,
			stats.SuccessRequests,
			stats.FailedRequests,
			stats.SentTransactions,
			stats.Duration.Truncate(time.Second),
		)
		fmt.Printf("\n✅ All tasks completed successfully!\n")
		fmt.Printf("💡 Press Enter to return to main menu...")

		// Wait for user input
		bufio.NewReader(os.Stdin).ReadLine()

		c.isRunning = false // Stop other monitoring
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

// handleManageAccountAuthentication manages account authentication
func (c *CLI) handleManageAccountAuthentication() {
	fmt.Println("🔐 Account Authentication Management")
	fmt.Println(strings.Repeat("-", 80))

	// Check account statuses
	accountStatuses := c.checkAccountStatuses()

	for {
		c.printAccountStatuses(accountStatuses)

		fmt.Println("\nOptions:")
		fmt.Println("1. 🔄 Authenticate selected accounts")
		fmt.Println("2. 🔄 Authenticate all accounts")
		fmt.Println("3. 📋 Refresh account statuses")
		fmt.Println("4. 🔙 Back to main menu")

		fmt.Print("Select option (1-4): ")
		reader := bufio.NewReader(os.Stdin)
		input, _ := reader.ReadString('\n')
		choice := strings.TrimSpace(input)

		switch choice {
		case "1":
			c.handleSelectiveAuthentication(&accountStatuses)
		case "2":
			c.handleAuthenticateAllAccounts(&accountStatuses)
		case "3":
			accountStatuses = c.checkAccountStatuses()
			fmt.Println("✅ Account statuses refreshed")
		case "4":
			return
		default:
			fmt.Println("❌ Invalid choice. Please try again.")
		}

		fmt.Println() // Add spacing
	}
}

// AccountStatus represents the authentication status of an account
type AccountStatus struct {
	Index        int
	Name         string
	PhoneNumber  string
	HasAuthToken bool
	HasSession   bool
	IsActive     bool
	Error        string
}

// checkAccountStatuses checks the authentication status of all accounts
func (c *CLI) checkAccountStatuses() []AccountStatus {
	var statuses []AccountStatus

	for i, account := range c.config.Accounts {
		status := AccountStatus{
			Index:        i,
			Name:         account.Name,
			PhoneNumber:  account.PhoneNumber,
			HasAuthToken: account.AuthToken != "",
		}

		// Check if session file exists - look in multiple possible locations
		if account.PhoneNumber != "" {
			// Clean phone number (remove + and other characters for file names)
			cleanPhone := strings.ReplaceAll(account.PhoneNumber, "+", "")

			// Try different session file patterns and locations
			possiblePaths := []string{
				// Current directory patterns with original phone
				fmt.Sprintf("sessions/%s.session", account.PhoneNumber),
				fmt.Sprintf("session/%s.session", account.PhoneNumber),
				fmt.Sprintf("%s.session", account.PhoneNumber),
				fmt.Sprintf("sessions/%s", account.PhoneNumber),
				fmt.Sprintf("session/%s", account.PhoneNumber),
				// Current directory patterns with clean phone (without +)
				fmt.Sprintf("sessions/%s.session", cleanPhone),
				fmt.Sprintf("session/%s.session", cleanPhone),
				fmt.Sprintf("%s.session", cleanPhone),
				fmt.Sprintf("sessions/%s", cleanPhone),
				fmt.Sprintf("session/%s", cleanPhone),
				// bin directory patterns (where exe is located) with original phone
				fmt.Sprintf("bin/sessions/%s.session", account.PhoneNumber),
				fmt.Sprintf("bin/session/%s.session", account.PhoneNumber),
				fmt.Sprintf("bin/%s.session", account.PhoneNumber),
				fmt.Sprintf("bin/sessions/%s", account.PhoneNumber),
				fmt.Sprintf("bin/session/%s", account.PhoneNumber),
				// bin directory patterns with clean phone
				fmt.Sprintf("bin/sessions/%s.session", cleanPhone),
				fmt.Sprintf("bin/session/%s.session", cleanPhone),
				fmt.Sprintf("bin/%s.session", cleanPhone),
				fmt.Sprintf("bin/sessions/%s", cleanPhone),
				fmt.Sprintf("bin/session/%s", cleanPhone),
				// Relative to exe location
				fmt.Sprintf("./sessions/%s.session", account.PhoneNumber),
				fmt.Sprintf("./session/%s.session", account.PhoneNumber),
				fmt.Sprintf("./%s.session", account.PhoneNumber),
				fmt.Sprintf("./sessions/%s.session", cleanPhone),
				fmt.Sprintf("./session/%s.session", cleanPhone),
				fmt.Sprintf("./%s.session", cleanPhone),
			}

			for _, path := range possiblePaths {
				if _, err := os.Stat(path); err == nil {
					status.HasSession = true
					break
				}
			}
		}

		// Determine if account is active (has either auth token or session)
		status.IsActive = status.HasAuthToken || status.HasSession

		// Check for potential issues
		if account.PhoneNumber == "" && account.AuthToken == "" {
			status.Error = "No phone number or auth token specified"
		} else if account.PhoneNumber != "" && !strings.HasPrefix(account.PhoneNumber, "+") {
			status.Error = "Phone number must start with '+'"
		}

		statuses = append(statuses, status)
	}

	return statuses
}

// printAccountStatuses displays the status of all accounts
func (c *CLI) printAccountStatuses(statuses []AccountStatus) {
	fmt.Println("\n📋 Account Authentication Status:")
	fmt.Println(strings.Repeat("-", 80))

	for _, status := range statuses {
		account := c.config.Accounts[status.Index]
		fmt.Printf("Account %d: %s\n", status.Index+1, status.Name)

		if status.PhoneNumber != "" {
			fmt.Printf("   📱 Phone: %s\n", maskPhoneNumber(status.PhoneNumber))
		} else {
			fmt.Printf("   📱 Phone: Not specified\n")
		}

		// Auth token status
		if status.HasAuthToken {
			fmt.Printf("   🎫 Auth Token: ✅ Available\n")
		} else {
			fmt.Printf("   🎫 Auth Token: ❌ Not available\n")
		}

		// Session status with debug info
		if status.HasSession {
			fmt.Printf("   📁 Session: ✅ Active\n")
		} else {
			fmt.Printf("   📁 Session: ❌ Not found\n")
			// Show where we looked for sessions (debug info)
			if status.PhoneNumber != "" {
				cleanPhone := strings.ReplaceAll(status.PhoneNumber, "+", "")
				fmt.Printf("   🔍 Searched for: %s.session, %s.session\n", status.PhoneNumber, cleanPhone)
			}
		}

		// Proxy status
		if account.UseProxy && account.ProxyURL != "" {
			// Mask proxy for security (show first part of host)
			maskedProxy := maskProxyURL(account.ProxyURL)
			fmt.Printf("   🌐 Proxy: ✅ Enabled (%s)\n", maskedProxy)
		} else if account.UseProxy && account.ProxyURL == "" {
			fmt.Printf("   🌐 Proxy: ⚠️  Enabled but URL not set\n")
		} else {
			fmt.Printf("   🌐 Proxy: ❌ Disabled\n")
		}

		// Overall status
		if status.IsActive {
			fmt.Printf("   🟢 Status: ACTIVE\n")
		} else {
			fmt.Printf("   🔴 Status: INACTIVE\n")
		}

		// Error if any
		if status.Error != "" {
			fmt.Printf("   ⚠️  Issue: %s\n", status.Error)
		}

		fmt.Println()
	}
}

// handleSelectiveAuthentication allows user to select which accounts to authenticate
func (c *CLI) handleSelectiveAuthentication(accountStatuses *[]AccountStatus) {
	fmt.Println("🎯 Select accounts to authenticate:")
	fmt.Println("Enter account numbers separated by commas (e.g., 1,3,5) or 'all' for all accounts")

	// Show inactive accounts
	inactiveAccounts := []AccountStatus{}
	for _, status := range *accountStatuses {
		if !status.IsActive && status.Error == "" {
			inactiveAccounts = append(inactiveAccounts, status)
		}
	}

	if len(inactiveAccounts) == 0 {
		fmt.Println("✅ All accounts are already active!")
		fmt.Print("Press Enter to continue...")
		bufio.NewReader(os.Stdin).ReadLine()
		return
	}

	fmt.Println("\nInactive accounts:")
	for _, status := range inactiveAccounts {
		fmt.Printf("  %d. %s (%s)\n", status.Index+1, status.Name, maskPhoneNumber(status.PhoneNumber))
	}

	fmt.Print("\nEnter your choice: ")
	reader := bufio.NewReader(os.Stdin)
	input, _ := reader.ReadString('\n')
	choice := strings.TrimSpace(input)

	if choice == "" {
		return
	}

	var selectedIndices []int

	if strings.ToLower(choice) == "all" {
		for _, status := range inactiveAccounts {
			selectedIndices = append(selectedIndices, status.Index)
		}
	} else {
		// Parse comma-separated numbers
		parts := strings.Split(choice, ",")
		for _, part := range parts {
			part = strings.TrimSpace(part)
			if num, err := strconv.Atoi(part); err == nil && num >= 1 && num <= len(*accountStatuses) {
				selectedIndices = append(selectedIndices, num-1)
			}
		}
	}

	if len(selectedIndices) == 0 {
		fmt.Println("❌ No valid accounts selected")
		return
	}

	// Authenticate selected accounts
	c.authenticateSelectedAccounts(selectedIndices)

	// Refresh statuses after authentication
	*accountStatuses = c.checkAccountStatuses()
	fmt.Println("📋 Account statuses refreshed after authentication")
}

// handleAuthenticateAllAccounts authenticates all inactive accounts
func (c *CLI) handleAuthenticateAllAccounts(accountStatuses *[]AccountStatus) {
	fmt.Println("🔄 Authenticating all accounts...")

	ctx := context.Background()
	if err := c.authIntegration.AuthorizeAccounts(ctx); err != nil {
		fmt.Printf("❌ Authentication error: %v\n", err)
	} else {
		fmt.Println("✅ All accounts authenticated successfully!")
	}

	// Refresh statuses after authentication
	*accountStatuses = c.checkAccountStatuses()
	fmt.Println("📋 Account statuses refreshed after authentication")

	fmt.Print("Press Enter to continue...")
	bufio.NewReader(os.Stdin).ReadLine()
}

// authenticateSelectedAccounts authenticates specific accounts by their indices
func (c *CLI) authenticateSelectedAccounts(indices []int) {
	fmt.Printf("🔄 Authenticating %d selected accounts...\n", len(indices))

	ctx := context.Background()
	successCount := 0

	// ИСПРАВЛЕНИЕ: НЕ создаем tempAuthIntegration который портит конфиг!
	// Вместо этого аутентифицируем аккаунты через основной authIntegration
	// но только выбранные индексы

	// Сохраняем оригинальные токены
	originalTokens := make(map[int]string)
	for _, index := range indices {
		if index >= 0 && index < len(c.config.Accounts) {
			originalTokens[index] = c.config.Accounts[index].AuthToken
		}
	}

	for _, index := range indices {
		if index < 0 || index >= len(c.config.Accounts) {
			continue
		}

		account := c.config.Accounts[index]
		fmt.Printf("🔐 Authenticating %s (%s)...\n", account.Name, maskPhoneNumber(account.PhoneNumber))

		// Временно очищаем токен чтобы authIntegration попытался аутентифицировать
		c.config.Accounts[index].AuthToken = ""

		// Используем основной authIntegration для аутентификации всех аккаунтов
		// но только те что нуждаются в аутентификации (без токенов) будут обработаны
		if err := c.authIntegration.AuthorizeAccounts(ctx); err != nil {
			fmt.Printf("❌ Failed to authenticate %s: %v\n", account.Name, err)
			// Восстанавливаем оригинальный токен при ошибке
			c.config.Accounts[index].AuthToken = originalTokens[index]
		} else {
			fmt.Printf("✅ Successfully authenticated %s\n", account.Name)
			successCount++
		}

		// Восстанавливаем токены других аккаунтов (которые не должны были аутентифицироваться)
		for otherIndex, originalToken := range originalTokens {
			if otherIndex != index && originalToken != "" {
				c.config.Accounts[otherIndex].AuthToken = originalToken
			}
		}
	}

	fmt.Printf("📊 Authentication complete: %d/%d accounts successful\n", successCount, len(indices))
	fmt.Print("Press Enter to continue...")
	bufio.NewReader(os.Stdin).ReadLine()
}

// handleCheckDeployWallets handles checking and deploying wallets
func (c *CLI) handleCheckDeployWallets() {
	fmt.Println("🔧 Checking/Deploying Wallets")
	fmt.Println(strings.Repeat("-", 80))

	ctx := context.Background()
	reader := bufio.NewReader(os.Stdin)

	var deployRequired []int

	fmt.Println("🔍 Scanning wallet states for all accounts...\n")

	// Check all accounts
	for i, account := range c.config.Accounts {
		fmt.Printf("Account %d: %s\n", i+1, account.Name)

		// Check if seed phrase is configured
		if account.SeedPhrase == "" {
			fmt.Printf("   ⚠️  No seed phrase configured - skipping\n\n")
			continue
		}

		// Validate seed phrase format
		words := strings.Fields(account.SeedPhrase)
		if len(words) != 12 && len(words) != 24 {
			fmt.Printf("   ❌ Invalid seed phrase format - skipping\n\n")
			continue
		}

		// Create TON client
		tonClient, err := client.NewTONClient(account.SeedPhrase)
		if err != nil {
			fmt.Printf("   ❌ Error creating TON client: %v\n\n", err)
			continue
		}

		// Get wallet address
		address := tonClient.GetAddress()
		fmt.Printf("   📍 Address: %s\n", address.String())

		// Get balance and check deployment status
		balance, err := tonClient.GetBalance(ctx)
		if err != nil {
			fmt.Printf("   ❌ Error getting balance: %v\n\n", err)
			continue
		}

		// Convert balance to TON
		balanceTON := new(big.Float).SetInt(balance)
		balanceTON.Quo(balanceTON, big.NewFloat(1e9))
		balanceFloat, _ := balanceTON.Float64()

		fmt.Printf("   💰 Balance: %.4f TON\n", balanceFloat)

		// Check if wallet is deployed by trying to get seqno
		deployed := c.isWalletDeployed(ctx, tonClient)
		if deployed {
			fmt.Printf("   ✅ Wallet is deployed and ready\n\n")
		} else {
			fmt.Printf("   ⚠️  Wallet is NOT deployed - requires deployment\n")
			if balanceFloat >= 0.05 {
				fmt.Printf("   💡 Balance sufficient for deployment (>= 0.05 TON)\n")
				deployRequired = append(deployRequired, i)
			} else {
				fmt.Printf("   ❌ Insufficient balance for deployment (need >= 0.05 TON)\n")
			}
			fmt.Println()
		}
	}

	// Show deployment options if needed
	if len(deployRequired) > 0 {
		fmt.Printf("🚀 Found %d wallets that need deployment\n\n", len(deployRequired))

		for {
			fmt.Println("Deployment options:")
			fmt.Println("1. 🔄 Deploy selected wallets")
			fmt.Println("2. 🔄 Deploy all undeployed wallets")
			fmt.Println("3. 📋 Refresh wallet statuses")
			fmt.Println("4. 🔙 Back to main menu")

			fmt.Print("Select option (1-4): ")
			input, _ := reader.ReadString('\n')
			choice := strings.TrimSpace(input)

			switch choice {
			case "1":
				c.handleSelectiveDeployment(deployRequired)
				return
			case "2":
				c.deployWallets(deployRequired)
				return
			case "3":
				c.handleCheckDeployWallets() // Recursive call to refresh
				return
			case "4":
				return
			default:
				fmt.Println("❌ Invalid choice. Please try again.")
			}
		}
	} else {
		fmt.Println("✅ All configured wallets are deployed and ready!")
		fmt.Print("Press Enter to continue...")
		reader.ReadLine()
	}
}

// isWalletDeployed checks if wallet is deployed by attempting a test transaction
func (c *CLI) isWalletDeployed(ctx context.Context, tonClient *client.TONClient) bool {
	// Try to send a minimal transaction to self to test deployment
	// If wallet is not deployed, this will automatically deploy it
	address := tonClient.GetAddress()

	// Create a test transaction with minimal amount (0.001 TON)
	result, err := tonClient.SendTON(ctx, address.String(), 1000000, "🔍 Deployment check", true, address.String())
	if err != nil {
		// If there's an error, assume wallet is not deployed
		return false
	}

	// If transaction was successful, wallet is deployed
	return result.Success
}

// handleSelectiveDeployment handles selective wallet deployment
func (c *CLI) handleSelectiveDeployment(deployRequired []int) {
	reader := bufio.NewReader(os.Stdin)

	fmt.Println("📝 Wallets requiring deployment:")
	for i, accountIndex := range deployRequired {
		account := c.config.Accounts[accountIndex]
		fmt.Printf("%d. %s (Account %d)\n", i+1, account.Name, accountIndex+1)
	}

	fmt.Print("\nEnter wallet numbers to deploy (e.g., 1,3,5) or 'all' for all: ")
	input, _ := reader.ReadString('\n')
	selection := strings.TrimSpace(input)

	var selectedIndices []int

	if strings.ToLower(selection) == "all" {
		selectedIndices = deployRequired
	} else {
		// Parse selected numbers
		parts := strings.Split(selection, ",")
		for _, part := range parts {
			num, err := strconv.Atoi(strings.TrimSpace(part))
			if err != nil || num < 1 || num > len(deployRequired) {
				fmt.Printf("❌ Invalid selection: %s\n", part)
				continue
			}
			selectedIndices = append(selectedIndices, deployRequired[num-1])
		}
	}

	if len(selectedIndices) == 0 {
		fmt.Println("❌ No valid wallets selected")
		return
	}

	c.deployWallets(selectedIndices)
}

// deployWallets deploys the specified wallets
func (c *CLI) deployWallets(accountIndices []int) {
	fmt.Printf("🚀 Starting deployment for %d wallets...\n\n", len(accountIndices))

	ctx := context.Background()
	successCount := 0

	for _, accountIndex := range accountIndices {
		account := c.config.Accounts[accountIndex]
		fmt.Printf("🔄 Deploying wallet for %s...\n", account.Name)

		// Create TON client
		tonClient, err := client.NewTONClient(account.SeedPhrase)
		if err != nil {
			fmt.Printf("   ❌ Error creating TON client: %v\n\n", err)
			continue
		}

		// The deployment will be handled automatically by the TON client
		// when first transaction is attempted. We can trigger this by
		// sending a small amount to self

		address := tonClient.GetAddress()
		fmt.Printf("   📍 Wallet address: %s\n", address.String())

		// Send deployment transaction (0.001 TON to self)
		result, err := tonClient.SendTON(ctx, address.String(), 1000000, "🚀 Wallet deployment", c.config.TestMode, c.config.TestAddress)
		if err != nil {
			fmt.Printf("   ❌ Deployment failed: %v\n\n", err)
			continue
		}

		if result.Success {
			fmt.Printf("   ✅ Wallet deployed successfully!\n")
			fmt.Printf("   📊 Transaction ID: %s\n\n", result.TransactionID)
			successCount++
		} else {
			fmt.Printf("   ❌ Deployment failed\n\n")
		}
	}

	fmt.Printf("🎉 Deployment completed! Success: %d/%d\n", successCount, len(accountIndices))
	fmt.Print("Press Enter to continue...")
	bufio.NewReader(os.Stdin).ReadLine()
}

// maskProxyURL masks proxy URL for display
func maskProxyURL(url string) string {
	if len(url) < 4 {
		return strings.Repeat("*", len(url))
	}
	return url[:3] + strings.Repeat("*", len(url)-6) + url[len(url)-3:]
}
