// OpenCompat provides a personal API compatibility layer.
//
// This is an independent open-source project for personal, non-commercial use.
// It is NOT affiliated with, endorsed by, or sponsored by OpenAI or any other company.
package main

import (
	"bufio"
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/edgard/opencompat/internal/auth"
	"github.com/edgard/opencompat/internal/config"
	"github.com/edgard/opencompat/internal/server"
)

var (
	version = "dev"
	commit  = "none"
	date    = "unknown"
)

const usage = `OpenCompat - Personal API compatibility layer

Usage:
  opencompat [command]

Commands:
  login    Authenticate with your account
  logout   Remove stored credentials
  info     Show authentication status
  serve    Start the API server (default)
  version  Show version information
  help     Show this help message

Environment Variables:
  OPENCOMPAT_HOST                Server bind address (default: 127.0.0.1)
  OPENCOMPAT_PORT                Server listen port (default: 8080)
  OPENCOMPAT_VERBOSE             Enable verbose logging (default: false)
  OPENCOMPAT_REASONING_EFFORT    Reasoning effort: none, low, medium, high, xhigh (default: medium)
  OPENCOMPAT_REASONING_SUMMARY   Reasoning summary: auto, concise, detailed (default: auto)
  OPENCOMPAT_TEXT_VERBOSITY      Text verbosity: low, medium, high (default: medium)
  OPENCOMPAT_INSTRUCTIONS_REFRESH  Instructions refresh interval in minutes (default: 1440 = 24h)
  OPENCOMPAT_OAUTH_CLIENT_ID     OAuth client ID (advanced, uses default if not set)
`

const acknowledgmentFile = "acknowledged"

const disclaimerText = `
================================================================================
                                    NOTICE
================================================================================

This is an independent open-source project for PERSONAL, NON-COMMERCIAL USE.
It is NOT affiliated with, endorsed by, or sponsored by OpenAI or any other
company.

By using this software, you acknowledge that:

1. You are responsible for compliance with all applicable terms of service.

2. You assume all risk for any consequences of your use.

3. The author is not liable for any damages arising from your use.

4. This software is provided "AS IS" without warranty of any kind.

For full terms, see the LICENSE file.

================================================================================
`

func main() {
	if len(os.Args) < 2 {
		cmdServe()
		return
	}

	switch os.Args[1] {
	case "login":
		cmdLogin()
	case "logout":
		cmdLogout()
	case "info":
		cmdInfo()
	case "serve":
		cmdServe()
	case "version", "-v", "--version":
		fmt.Printf("opencompat %s (commit: %s, built: %s)\n", version, commit, date)
	case "help", "-h", "--help":
		fmt.Print(usage)
	default:
		fmt.Fprintf(os.Stderr, "Unknown command: %s\n\n", os.Args[1])
		fmt.Print(usage)
		os.Exit(1)
	}
}

func cmdLogin() {
	store := auth.NewStore()
	cfg := config.Load()

	if err := auth.Login(store, cfg); err != nil {
		fmt.Fprintf(os.Stderr, "Login failed: %v\n", err)
		os.Exit(1)
	}
}

func cmdLogout() {
	store := auth.NewStore()

	if err := store.Clear(); err != nil {
		fmt.Fprintf(os.Stderr, "Logout failed: %v\n", err)
		os.Exit(1)
	}

	fmt.Println("Logged out successfully.")
}

func cmdInfo() {
	store := auth.NewStore()

	if err := store.Load(); err != nil {
		fmt.Fprintf(os.Stderr, "Not logged in: %v\n", err)
		os.Exit(1)
	}

	bundle, err := store.GetBundle()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to get auth info: %v\n", err)
		os.Exit(1)
	}

	fmt.Println("Authentication Status: Logged in")
	if bundle.Email != "" {
		fmt.Printf("Email: %s\n", bundle.Email)
	}
	if bundle.AccountID != "" {
		fmt.Printf("Account ID: %s\n", bundle.AccountID)
	}
	fmt.Printf("Token Expires: %s\n", bundle.ExpiresAt.Format("2006-01-02 15:04:05"))

	if bundle.IsExpired() {
		fmt.Println("Status: Token expired (will refresh on next request)")
	} else {
		fmt.Println("Status: Token valid")
	}
}

func cmdServe() {
	// Check acknowledgment first
	if err := checkAcknowledgment(); err != nil {
		fmt.Fprintf(os.Stderr, "%v\n", err)
		os.Exit(1)
	}

	store := auth.NewStore()

	// Check if logged in
	if !store.IsLoggedIn() {
		fmt.Fprintln(os.Stderr, "Not logged in. Please run 'opencompat login' first.")
		os.Exit(1)
	}

	cfg := config.Load()
	srv := server.New(store, cfg)

	// Prefetch instructions before starting server
	// This ensures all model instructions are available
	if err := srv.PrefetchInstructions(); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to prefetch instructions: %v\n", err)
		fmt.Fprintln(os.Stderr, "Server cannot start without instructions.")
		os.Exit(1)
	}

	// Setup signal handling for graceful shutdown
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	// Start server in goroutine
	errChan := make(chan error, 1)
	go func() {
		errChan <- srv.Start()
	}()

	// Wait for signal or error
	select {
	case sig := <-sigChan:
		log.Printf("Received signal %v, shutting down...", sig)
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		if err := srv.Shutdown(ctx); err != nil {
			log.Printf("Shutdown error: %v", err)
		}
		log.Println("Server stopped")
	case err := <-errChan:
		if err != nil {
			fmt.Fprintf(os.Stderr, "Server error: %v\n", err)
			os.Exit(1)
		}
	}
}

func checkAcknowledgment() error {
	ackPath := filepath.Join(config.DataDir(), acknowledgmentFile)

	// Check if already acknowledged
	if _, err := os.Stat(ackPath); err == nil {
		return nil
	}

	// Display disclaimer
	fmt.Print(disclaimerText)
	fmt.Print("\nDo you understand and agree to these terms? (yes/no): ")

	reader := bufio.NewReader(os.Stdin)
	response, err := reader.ReadString('\n')
	if err != nil {
		return fmt.Errorf("failed to read response: %w", err)
	}

	response = strings.TrimSpace(strings.ToLower(response))
	if response != "yes" {
		return fmt.Errorf("you must agree to the terms to use this software")
	}

	// Save acknowledgment
	if err := config.EnsureDataDir(); err != nil {
		return fmt.Errorf("failed to create data directory: %w", err)
	}

	ackContent := fmt.Sprintf("Acknowledged: %s\n", time.Now().Format(time.RFC3339))
	if err := os.WriteFile(ackPath, []byte(ackContent), 0600); err != nil {
		return fmt.Errorf("failed to save acknowledgment: %w", err)
	}

	fmt.Println("\nThank you. Starting server...")
	return nil
}
