package main

import (
	"context"
	"crypto/tls"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/idp/tunnel/pkg/tunnel"
	"go.uber.org/zap"
	"gopkg.in/yaml.v3"
)

var (
	listenAddr  = flag.String("listen", ":8443", "Server listen address")
	certFile    = flag.String("cert", "", "TLS certificate file")
	keyFile     = flag.String("key", "", "TLS key file")
	authToken   = flag.String("token", "", "Authentication token (required)")
	useImproved = flag.Bool("improved", true, "Use improved implementation with better reliability")
	configFile  = flag.String("config", getConfigPath(), "Configuration file path")
)

// ForwarderConfig moved to tunnel package

type ServerConfig struct {
	Listen   string `yaml:"listen"`
	Token    string `yaml:"token"`
	Improved bool   `yaml:"improved"`
	TLS      struct {
		Cert string `yaml:"cert"`
		Key  string `yaml:"key"`
	} `yaml:"tls"`
}

type Config struct {
	Server     ServerConfig               `yaml:"server"`
	Forwarders []tunnel.ForwarderConfig `yaml:"forwarders"`
}

func getConfigPath() string {
	if path := os.Getenv("TUNNEL_CONFIG"); path != "" {
		return path
	}
	return "config.yaml"
}

// getEnvAsInt reads an environment variable and converts it to int, returns defaultValue if not set or invalid
func getEnvAsInt(key string, defaultValue int) int {
	if value := os.Getenv(key); value != "" {
		if intValue, err := strconv.Atoi(value); err == nil {
			return intValue
		}
		log.Printf("Warning: Invalid value for %s: %s, using default %d", key, value, defaultValue)
	}
	return defaultValue
}

// getEnvAsBool reads an environment variable and converts it to bool, returns defaultValue if not set or invalid
func getEnvAsBool(key string, defaultValue bool) bool {
	if value := os.Getenv(key); value != "" {
		if boolValue, err := strconv.ParseBool(value); err == nil {
			return boolValue
		}
		log.Printf("Warning: Invalid value for %s: %s, using default %t", key, value, defaultValue)
	}
	return defaultValue
}

func expandEnvVars(s string) string {
	return os.ExpandEnv(s)
}

func loadConfig(configPath string, logger *zap.Logger) (*Config, error) {
	// Set default config
	config := &Config{
		Server: ServerConfig{
			Listen:   ":8443",
			Token:    "${TUNNEL_TOKEN}",
			Improved: true,
		},
	}
	
	// Try to load config file
	if _, err := os.Stat(configPath); err == nil {
		data, err := os.ReadFile(configPath)
		if err != nil {
			return nil, fmt.Errorf("failed to read config file: %w", err)
		}
		
		if err := yaml.Unmarshal(data, config); err != nil {
			return nil, fmt.Errorf("failed to parse config file: %w", err)
		}
		
		logger.Info("Loaded configuration from file", zap.String("path", configPath))
	} else {
		logger.Warn("Config file not found, using defaults", zap.String("path", configPath))
	}
	
	// Expand environment variables
	config.Server.Listen = expandEnvVars(config.Server.Listen)
	config.Server.Token = expandEnvVars(config.Server.Token)
	config.Server.TLS.Cert = expandEnvVars(config.Server.TLS.Cert)
	config.Server.TLS.Key = expandEnvVars(config.Server.TLS.Key)
	
	for i := range config.Forwarders {
		config.Forwarders[i].Target = expandEnvVars(config.Forwarders[i].Target)
		config.Forwarders[i].ClientID = expandEnvVars(config.Forwarders[i].ClientID)
		
		// Apply environment variable overrides
		envPrefix := fmt.Sprintf("TUNNEL_FORWARDER_%s_", strings.ToUpper(config.Forwarders[i].Name))
		
		if portStr := os.Getenv(envPrefix + "PORT"); portStr != "" {
			if port, err := strconv.Atoi(portStr); err == nil {
				config.Forwarders[i].Port = port
			}
		}
		
		if target := os.Getenv(envPrefix + "TARGET"); target != "" {
			config.Forwarders[i].Target = target
		}
		
		if enabledStr := os.Getenv(envPrefix + "ENABLED"); enabledStr != "" {
			if enabled, err := strconv.ParseBool(enabledStr); err == nil {
				config.Forwarders[i].Enabled = enabled
			}
		}
	}
	
	return config, nil
}

func validatePortRange(port int) error {
	if port < 1 || port > 65535 {
		return fmt.Errorf("port %d is out of valid range (1-65535)", port)
	}
	return nil
}

func validateConfig(config *Config, logger *zap.Logger) []tunnel.ForwarderConfig {
	var validForwarders []tunnel.ForwarderConfig
	usedPorts := make(map[int]bool)
	
	for _, forwarder := range config.Forwarders {
		if !forwarder.Enabled {
			logger.Debug("Forwarder disabled", zap.String("name", forwarder.Name))
			continue
		}
		
		if err := validatePortRange(forwarder.Port); err != nil {
			logger.Error("Invalid port configuration", zap.String("name", forwarder.Name), zap.Error(err))
			continue
		}
		
		if usedPorts[forwarder.Port] {
			logger.Error("Port conflict detected", zap.String("name", forwarder.Name), zap.Int("port", forwarder.Port))
			continue
		}
		
		if forwarder.Target == "" {
			logger.Error("Target not specified", zap.String("name", forwarder.Name))
			continue
		}
		
		usedPorts[forwarder.Port] = true
		validForwarders = append(validForwarders, forwarder)
		logger.Info("Validated forwarder", 
			zap.String("name", forwarder.Name),
			zap.Int("port", forwarder.Port),
			zap.String("target", forwarder.Target))
	}
	
	return validForwarders
}

func startTCPForwarders(server any, configs []tunnel.ForwarderConfig, logger *zap.Logger, useImproved bool) {
	for _, config := range configs {
		if useImproved {
			if improvedServer, ok := server.(*tunnel.ImprovedServer); ok {
				if err := improvedServer.StartTCPForwarder(config.Port, config.ClientID); err != nil {
					if config.WarningOnFail {
						logger.Warn("Forwarder not started (may be expected)", 
							zap.String("name", config.Name), 
							zap.Int("port", config.Port), 
							zap.Error(err))
					} else {
						logger.Error("Failed to start forwarder", 
							zap.String("name", config.Name), 
							zap.Int("port", config.Port), 
							zap.String("target", config.Target),
							zap.Error(err))
					}
				} else {
					logger.Info("Started TCP forwarder", 
						zap.String("name", config.Name),
						zap.Int("port", config.Port), 
						zap.String("target", config.Target),
						zap.String("description", config.Description))
				}
			}
		} else {
			if originalServer, ok := server.(*tunnel.Server); ok {
				go func(config tunnel.ForwarderConfig) {
					originalServer.StartTCPForwarder(config.Port, config.ClientID)
					logger.Info("Started TCP forwarder", 
						zap.String("name", config.Name),
						zap.Int("port", config.Port),
						zap.String("target", config.Target))
				}(config)
			}
		}
	}
}

func main() {
	flag.Parse()

	logger, _ := zap.NewProduction()
	defer logger.Sync()

	// Load configuration
	config, err := loadConfig(*configFile, logger)
	if err != nil {
		logger.Fatal("Failed to load configuration", zap.Error(err))
	}
	
	// Override config with command line flags
	if *listenAddr != ":8443" {
		config.Server.Listen = *listenAddr
	}
	if *authToken != "" {
		config.Server.Token = *authToken
	}
	if *certFile != "" {
		config.Server.TLS.Cert = *certFile
	}
	if *keyFile != "" {
		config.Server.TLS.Key = *keyFile
	}
	
	// Validate required token
	if config.Server.Token == "" || config.Server.Token == "${TUNNEL_TOKEN}" {
		logger.Fatal("Authentication token is required (set in config file, -token flag, or TUNNEL_TOKEN env var)")
	}

	mux := http.NewServeMux()
	
	// Validate and filter forwarder configurations
	validConfigs := validateConfig(config, logger)
	
	// Create server instance based on implementation choice
	var server any
	implType := "improved"
	if config.Server.Improved {
		logger.Info("Using improved tunnel server implementation")
		server = tunnel.NewImprovedServer(logger, config.Server.Token, config.Forwarders)
		mux.HandleFunc("/tunnel", server.(*tunnel.ImprovedServer).HandleTunnel)
	} else {
		logger.Info("Using original tunnel server implementation")
		server = tunnel.NewServer(logger, config.Server.Token)
		mux.HandleFunc("/tunnel", server.(*tunnel.Server).HandleTunnel)
		implType = "original"
	}
	
	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(fmt.Sprintf(`{"status":"healthy","implementation":"%s","forwarders":%d}`, implType, len(validConfigs))))
	})

	// Start TCP forwarders using unified function
	startTCPForwarders(server, validConfigs, logger, config.Server.Improved)

	// Configure server with proper timeouts
	srv := &http.Server{
		Addr:         config.Server.Listen,
		Handler:      mux,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	if config.Server.TLS.Cert != "" && config.Server.TLS.Key != "" {
		srv.TLSConfig = &tls.Config{
			MinVersion: tls.VersionTLS12,
			CipherSuites: []uint16{
				tls.TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384,
				tls.TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256,
				tls.TLS_ECDHE_ECDSA_WITH_AES_256_GCM_SHA384,
				tls.TLS_ECDHE_ECDSA_WITH_AES_128_GCM_SHA256,
			},
		}
	}

	// Setup graceful shutdown
	go func() {
		sigChan := make(chan os.Signal, 1)
		signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)
		<-sigChan
		
		logger.Info("Shutting down server...")
		
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		
		if err := srv.Shutdown(ctx); err != nil {
			logger.Error("Server forced to shutdown", zap.Error(err))
		}
	}()

	logger.Info("Starting tunnel server", 
		zap.String("addr", config.Server.Listen),
		zap.Bool("improved", config.Server.Improved),
		zap.Int("forwarders", len(validConfigs)),
		zap.String("config", *configFile))
	
	var serverErr error
	if config.Server.TLS.Cert != "" && config.Server.TLS.Key != "" {
		serverErr = srv.ListenAndServeTLS(config.Server.TLS.Cert, config.Server.TLS.Key)
		logger.Info("Using TLS", zap.String("cert", config.Server.TLS.Cert))
	} else {
		logger.Warn("Running without TLS - not recommended for production")
		serverErr = srv.ListenAndServe()
	}
	
	if serverErr != nil && serverErr != http.ErrServerClosed {
		logger.Fatal("Server failed", zap.Error(serverErr))
	}
	
	logger.Info("Server stopped")
}