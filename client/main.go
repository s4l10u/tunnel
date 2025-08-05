package main

import (
	"context"
	"flag"
	"log"
	"os"
	"os/signal"
	"strings"
	"strconv"
	"syscall"
	"time"

	"github.com/idp/tunnel/pkg/tunnel"
	"go.uber.org/zap"
)

var (
	serverURL   = flag.String("server", "wss://localhost:8443/tunnel", "Tunnel server URL")
	authToken   = flag.String("token", "", "Authentication token (required)")
	clientID    = flag.String("id", "", "Client ID (optional)")
	skipVerify  = flag.Bool("skip-verify", false, "Skip TLS verification (dev only)")
	forward     = flag.String("forward", "", "Port forwarding config (e.g., '8080:localhost:80')")
	useImproved = flag.Bool("improved", true, "Use improved implementation with better reliability")
	showMetrics = flag.Bool("metrics", false, "Show connection metrics periodically")
)

func main() {
	flag.Parse()

	// Get auth token from env if not provided via flag
	if *authToken == "" {
		*authToken = os.Getenv("TUNNEL_TOKEN")
	}
	
	if *authToken == "" {
		log.Fatal("Authentication token is required (use -token flag or TUNNEL_TOKEN env var)")
	}

	logger, _ := zap.NewProduction()
	defer logger.Sync()

	// Skip-verify flag is handled by the TLS configuration in the WebSocket dialer
	// No need to modify the URL scheme

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Handle shutdown
	go func() {
		sigChan := make(chan os.Signal, 1)
		signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)
		<-sigChan
		logger.Info("Shutting down client...")
		cancel()
	}()

	// Use improved implementation by default
	if *useImproved {
		logger.Info("Using improved tunnel client implementation")
		
		config := tunnel.DefaultImprovedClientConfig(*serverURL, *authToken, *clientID, logger)
		config.SkipVerify = *skipVerify
		
		client := tunnel.NewImprovedClient(config)
		
		// Parse forward configuration for improved client
		if *forward != "" {
			// Parse format: "8088:target:443"
			parts := strings.Split(*forward, ":")
			if len(parts) == 3 {
				if port, err := strconv.Atoi(parts[0]); err == nil {
					target := parts[1] + ":" + parts[2]
					config.PortMappings[port] = target
					logger.Info("Configured port mapping", 
						zap.Int("port", port),
						zap.String("target", target))
				} else {
					logger.Error("Invalid port in forward configuration", 
						zap.String("forward", *forward))
				}
			} else {
				logger.Error("Invalid forward configuration format, expected port:host:port", 
					zap.String("forward", *forward))
			}
		}
		
		// Show metrics periodically if requested
		if *showMetrics {
			go func() {
				ticker := time.NewTicker(30 * time.Second)
				defer ticker.Stop()
				
				for {
					select {
					case <-ticker.C:
						metrics := client.GetMetrics()
						logger.Info("Client metrics",
							zap.Any("metrics", metrics))
					case <-ctx.Done():
						return
					}
				}
			}()
		}
		
		// Start the client
		logger.Info("Starting improved tunnel client", zap.String("server", *serverURL))
		if err := client.Start(ctx); err != nil {
			logger.Fatal("Failed to start client", zap.Error(err))
		}
	} else {
		logger.Info("Using original tunnel client implementation")
		
		config := tunnel.ClientConfig{
			ServerURL:  *serverURL,
			AuthToken:  *authToken,
			ClientID:   *clientID,
			SkipVerify: *skipVerify,
			Logger:     logger,
		}

		client := tunnel.NewClient(config)

		// Store port forwarding configuration if specified
		if *forward != "" {
			parts := strings.Split(*forward, ":")
			if len(parts) != 3 {
				logger.Fatal("Invalid forward format. Use 'localPort:remoteHost:remotePort'")
			}
			
			localPort, err := strconv.Atoi(parts[0])
			if err != nil {
				logger.Fatal("Invalid local port", zap.Error(err))
			}
			
			remotePort, err := strconv.Atoi(parts[2])
			if err != nil {
				logger.Fatal("Invalid remote port", zap.Error(err))
			}
			
			logger.Info("Configured port forwarding", 
				zap.Int("remotePort", localPort),
				zap.String("localHost", parts[1]),
				zap.Int("localPort", remotePort))
		}

		// Start the client
		logger.Info("Starting tunnel client", zap.String("server", *serverURL))
		if err := client.Start(ctx); err != nil {
			logger.Fatal("Failed to start client", zap.Error(err))
		}
	}
}