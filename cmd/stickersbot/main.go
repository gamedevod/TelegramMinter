package main

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"stickersbot/internal/config"
	"stickersbot/internal/service"
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

		fmt.Print("Select menu option (1-5): ")
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
	fmt.Println("5. 🚪 Exit")
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
