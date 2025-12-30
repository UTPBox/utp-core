package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"

	"github.com/sagernet/sing-box"
	"github.com/sagernet/sing-box/option"
	"github.com/spf13/cobra"

	_ "github.com/UTPBox/utp-core/extensions/psiphon"
)

var (
	version = "dev"
	commit  = "unknown"
)

var rootCmd = &cobra.Command{
	Use:   "utp-core",
	Short: "UTP-Core - Universal Tunnel Protocol Core",
	Long:  `UTP-Core is a proxy core based on Sing-box, designed for advanced networking capabilities.`,
}

var runCmd = &cobra.Command{
	Use:   "run",
	Short: "Run the UTP-Core service",
	RunE:  runService,
}

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Print version information",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Printf("UTP-Core %s (commit: %s)\n", version, commit)
	},
}

var configPath string

func init() {
	runCmd.Flags().StringVarP(&configPath, "config", "c", "config.json", "Path to configuration file")
	rootCmd.AddCommand(runCmd)
	rootCmd.AddCommand(versionCmd)
}

func main() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

func runService(cmd *cobra.Command, args []string) error {
	// Load configuration
	configContent, err := os.ReadFile(configPath)
	if err != nil {
		return fmt.Errorf("failed to read config file: %w", err)
	}

	// Parse configuration
	var options option.Options
	err = options.UnmarshalJSON(configContent)
	if err != nil {
		return fmt.Errorf("failed to parse config: %w", err)
	}

	// Set up logging
	if options.Log == nil {
		options.Log = &option.LogOptions{
			Level:  "info",
			Output: filepath.Join(os.TempDir(), "utp-core.log"),
		}
	}

	// Create context
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Create Sing-box instance
	instance, err := box.New(box.Options{
		Context: ctx,
		Options: options,
	})
	if err != nil {
		return fmt.Errorf("failed to create instance: %w", err)
	}

	// Start the service
	err = instance.Start()
	if err != nil {
		return fmt.Errorf("failed to start service: %w", err)
	}

	fmt.Println("UTP-Core started successfully")
	fmt.Printf("Configuration: %s\n", configPath)
	fmt.Println("Press Ctrl+C to stop")

	// Wait for interrupt signal
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM)
	<-sigCh

	fmt.Println("\nShutting down...")

	// Close the instance
	err = instance.Close()
	if err != nil {
		return fmt.Errorf("failed to stop service: %w", err)
	}

	fmt.Println("UTP-Core stopped")
	return nil
}
