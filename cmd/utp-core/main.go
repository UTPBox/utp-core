package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"

	"github.com/sagernet/sing-box"
	"github.com/sagernet/sing-box/adapter/outbound"
	"github.com/sagernet/sing-box/include"
	"github.com/sagernet/sing-box/option"
	"github.com/spf13/cobra"

	psiphon "github.com/UTPBox/utp-core/extensions/psiphon"
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
	// 1. Load configuration file
	configContent, err := os.ReadFile(configPath)
	if err != nil {
		return fmt.Errorf("failed to read config file: %w", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// 2. Initialize Registries using include package
	inboundRegistry := include.InboundRegistry()
	outboundRegistry := include.OutboundRegistry()
	endpointRegistry := include.EndpointRegistry()
	dnsTransportRegistry := include.DNSTransportRegistry()
	serviceRegistry := include.ServiceRegistry()

	// 3. Register Custom Psiphon Outbound
	outbound.Register[psiphon.PsiphonOptions](outboundRegistry, "psiphon", psiphon.NewOutbound)

	// 4. Inject Registries into Context
	ctx = box.Context(
		ctx,
		inboundRegistry,
		outboundRegistry,
		endpointRegistry,
		dnsTransportRegistry,
		serviceRegistry,
	)

	// 5. Parse configuration contextually (Required for custom protocols)
	var options option.Options
	err = options.UnmarshalJSONContext(ctx, configContent)
	if err != nil {
		return fmt.Errorf("failed to parse config: %w", err)
	}

	// 6. Set up default logging if missing (optional)
	if options.Log == nil {
		options.Log = &option.LogOptions{
			Level:  "info",
			Output: filepath.Join(os.TempDir(), "utp-core.log"),
		}
	}

	// 7. Create and Start Sing-box instance
	instance, err := box.New(box.Options{
		Context: ctx,
		Options: options,
	})
	if err != nil {
		return fmt.Errorf("failed to create instance: %w", err)
	}

	if err := instance.Start(); err != nil {
		return fmt.Errorf("failed to start instance: %w", err)
	}
	defer instance.Close()

	fmt.Println("UTP-Core started successfully")
	
	// Wait for interrupt
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	<-sigCh

	return nil
}
