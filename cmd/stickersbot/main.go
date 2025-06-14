package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"stickersbot/internal/version"
	"syscall"
	"time"

	"stickersbot/internal/config"
	"stickersbot/internal/service"
)

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

	// Find configuration
	cfgPath := findConfigPath()
	cfg, err := config.Load(cfgPath)
	if err != nil {
		fmt.Printf("❌ Configuration loading error (%s): %v\n", cfgPath, err)
		os.Exit(1)
	}
	fmt.Printf("📋 Configuration loaded: %s\n", cfgPath)

	// Perform license authentication
	if version.Production {
		fmt.Println("🔐 Checking license...")
		if cfg.LicenseKey == "" {
			fmt.Println("❌ License key is not specified in config.json")
			os.Exit(1)
		}

		err := authenticate(cfg.LicenseKey)
		if err != nil {
			fmt.Printf("❌ License authentication failed: %v\n", err)
			os.Exit(1)
		}

		fmt.Println("✅ License authenticated successfully")
		startVerifier(cfg.LicenseKey)
	} else {
		fmt.Println("🧪 Running in development mode (license check disabled)")
	}

	// Validate configuration
	if !cfg.IsValid() {
		fmt.Println("❌ Configuration is invalid. Check accounts and their settings.")
		os.Exit(1)
	}

	// Create authorization service
	authIntegration := service.NewAuthIntegration(cfg)

	// Validate Telegram authorization settings
	if errors := authIntegration.ValidateAccounts(); len(errors) > 0 {
		fmt.Println("❌ Telegram authorization settings errors:")
		for _, err := range errors {
			fmt.Printf("   • %v\n", err)
		}
		os.Exit(1)
	}

	// Create sessions folder if it doesn't exist
	if err := os.MkdirAll("sessions", 0755); err != nil {
		fmt.Printf("❌ Error creating sessions folder: %v\n", err)
		os.Exit(1)
	}

	// Perform Telegram authorization for accounts that need it
	ctx := context.Background()
	if err := authIntegration.AuthorizeAccounts(ctx); err != nil {
		fmt.Printf("❌ Authorization error: %v\n", err)
		os.Exit(1)
	}

	// Create buyer service
	buyerService := service.NewBuyerService(cfg)

	// Start service
	if err := buyerService.Start(); err != nil {
		fmt.Printf("❌ Service startup error: %v\n", err)
		os.Exit(1)
	}

	fmt.Println("🚀 Sticker purchasing started! Press Ctrl+C to stop.")

	// Goroutine for broadcasting logs to stdout
	go func() {
		for log := range buyerService.GetLogChannel() {
			fmt.Println(log)
		}
	}()

	// Periodic statistics output every 5 seconds
	go func() {
		ticker := time.NewTicker(5 * time.Second)
		defer ticker.Stop()
		for range ticker.C {
			stats := buyerService.GetStatistics()
			fmt.Printf("📈 Total: %d | Success: %d | Errors: %d | TON sent: %d | RPS: %.1f | Time: %s\n",
				stats.TotalRequests,
				stats.SuccessRequests,
				stats.FailedRequests,
				stats.SentTransactions,
				stats.RequestsPerSec,
				stats.Duration.Truncate(time.Second),
			)
		}
	}()

	// Catch Ctrl+C / SIGTERM
	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, os.Interrupt, syscall.SIGTERM)
	<-sigs // Block until signal

	fmt.Println("\n🛑 Stopping...")
	buyerService.Stop()

	// Give workers time to finish gracefully
	time.Sleep(2 * time.Second)

	stats := buyerService.GetStatistics()
	fmt.Printf("✅ Completed. Total requests: %d, Success: %d, Errors: %d, TON sent: %d.\n",
		stats.TotalRequests, stats.SuccessRequests, stats.FailedRequests, stats.SentTransactions)
}

// findConfigPath returns the path to the configuration file
func findConfigPath() string {
	return "./config.json"
}
