package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/sagernet/sing-box"
	"github.com/sagernet/sing-box/common"
	"github.com/sagernet/sing-box/option"
	
	// Core extensions - import all supported protocols
	_ "github.com/UTPBox/utp-core/extensions/openvpn"
	_ "github.com/UTPBox/utp-core/extensions/ssh"
	_ "github.com/UTPBox/utp-core/extensions/dns"
	_ "github.com/UTPBox/utp-core/extensions/obfs"
	_ "github.com/UTPBox/utp-core/extensions/warp"
	_ "github.com/UTPBox/utp-core/extensions/psiphon"
	_ "github.com/UTPBox/utp-core/extensions/httpinject"
	_ "github.com/UTPBox/utp-core/extensions/stealth"
	_ "github.com/UTPBox/utp-core/extensions/legacyvpn"
	_ "github.com/UTPBox/utp-core/extensions/experimental"
)

func main() {
	// Set up signal handling for graceful shutdown
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		<-sigCh
		log.Println("Received shutdown signal, stopping UTP-Core...")
		cancel()
	}()

	// Load configuration
	var options option.Options
	
	// Check for config file
	configFile := "config.json"
	if len(os.Args) > 1 {
		configFile = os.Args[1]
	}

	if err := common.LoadConfig(ctx, &options, configFile); err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	// Create and start the instance
	instance, err := common.New(options)
	if err != nil {
		log.Fatalf("Failed to create instance: %v", err)
	}

	// Start the instance
	if err := instance.Start(); err != nil {
		log.Fatalf("Failed to start instance: %v", err)
	}

	log.Println("UTP-Core started successfully with all extensions loaded")
	log.Printf("Supported protocols: OpenVPN, SSH (all variants), DNS (DoH/DoT/DNSCrypt/DoQ), Obfuscation (Obfs4/Meek/Cloak), WARP, Psiphon, HTTP Injection, Stealth, Legacy VPN")
	
	// Wait for shutdown signal
	<-ctx.Done()
	
	// Graceful shutdown
	log.Println("Stopping UTP-Core...")
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer shutdownCancel()
	
	if err := instance.Stop(shutdownCtx); err != nil {
		log.Printf("Error during shutdown: %v", err)
	}
	
	log.Println("UTP-Core stopped successfully")
}
