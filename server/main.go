package main

import (
	"context"
	"crypto/tls"
	"flag"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"github.com/idp/tunnel/pkg/tunnel"
	"go.uber.org/zap"
)

var (
	listenAddr  = flag.String("listen", ":8443", "Server listen address")
	certFile    = flag.String("cert", "", "TLS certificate file")
	keyFile     = flag.String("key", "", "TLS key file")
	authToken   = flag.String("token", "", "Authentication token (required)")
	useImproved = flag.Bool("improved", true, "Use improved implementation with better reliability")
)

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

	mux := http.NewServeMux()
	
	// Use improved implementation by default
	if *useImproved {
		logger.Info("Using improved tunnel server implementation")
		server := tunnel.NewImprovedServer(logger, *authToken)
		
		mux.HandleFunc("/tunnel", server.HandleTunnel)
		mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`{"status":"healthy","implementation":"improved"}`))
		})
		
		// Configure TCP forwarder ports via environment variables
		webPort := getEnvAsInt("TUNNEL_WEB_PORT", 8080)
		dbPort := getEnvAsInt("TUNNEL_DB_PORT", 5432)
		sshPort := getEnvAsInt("TUNNEL_SSH_PORT", 2222)
		mongoPort := getEnvAsInt("TUNNEL_MONGO_PORT", 27017)
		k8sPort := getEnvAsInt("TUNNEL_K8S_PORT", 6443)
		
		// Control which forwarders to enable
		enableWeb := getEnvAsBool("TUNNEL_ENABLE_WEB", true)
		enableDB := getEnvAsBool("TUNNEL_ENABLE_DB", true)
		enableSSH := getEnvAsBool("TUNNEL_ENABLE_SSH", true)
		enableMongo := getEnvAsBool("TUNNEL_ENABLE_MONGO", true)
		enableK8s := getEnvAsBool("TUNNEL_ENABLE_K8S", true)
		
		// Start TCP forwarding listeners with error handling
		if enableWeb {
			if err := server.StartTCPForwarder(webPort, "airgap-web"); err != nil {
				logger.Error("Failed to start web forwarder", zap.Int("port", webPort), zap.Error(err))
			} else {
				logger.Info("Started TCP forwarder", zap.Int("port", webPort), zap.String("service", "web"))
			}
		}
		
		if enableDB {
			if err := server.StartTCPForwarder(dbPort, "airgap-db"); err != nil {
				logger.Error("Failed to start database forwarder", zap.Int("port", dbPort), zap.Error(err))
			} else {
				logger.Info("Started TCP forwarder", zap.Int("port", dbPort), zap.String("service", "database"))
			}
		}
		
		if enableSSH {
			if err := server.StartTCPForwarder(sshPort, "airgap-ssh"); err != nil {
				logger.Error("Failed to start SSH forwarder", zap.Int("port", sshPort), zap.Error(err))
			} else {
				logger.Info("Started TCP forwarder", zap.Int("port", sshPort), zap.String("service", "ssh"))
			}
		}
		
		if enableMongo {
			if err := server.StartTCPForwarder(mongoPort, "airgap-mongodb"); err != nil {
				logger.Error("Failed to start MongoDB forwarder", zap.Int("port", mongoPort), zap.Error(err))
			} else {
				logger.Info("Started TCP forwarder", zap.Int("port", mongoPort), zap.String("service", "mongodb"))
			}
		}
		
		if enableK8s {
			if err := server.StartTCPForwarder(k8sPort, "airgap-k8s-api"); err != nil {
				logger.Warn("Kubernetes API forwarder not started (port may be in use)", zap.Int("port", k8sPort), zap.Error(err))
			} else {
				logger.Info("Started TCP forwarder", zap.Int("port", k8sPort), zap.String("service", "kubernetes-api"))
			}
		}
	} else {
		logger.Info("Using original tunnel server implementation")
		server := tunnel.NewServer(logger, *authToken)
		
		mux.HandleFunc("/tunnel", server.HandleTunnel)
		mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`{"status":"healthy","implementation":"original"}`))
		})
		
		// Start TCP forwarding listeners for commonly used ports
		go server.StartTCPForwarder(8080, "airgap-web")     // Web app
		go server.StartTCPForwarder(5432, "airgap-db")      // Database
		go server.StartTCPForwarder(2222, "airgap-ssh")     // SSH
		go server.StartTCPForwarder(27017, "airgap-mongodb") // MongoDB
		go server.StartTCPForwarder(6443, "airgap-k8s-api") // Kubernetes API
	}

	// Configure server with proper timeouts
	srv := &http.Server{
		Addr:         *listenAddr,
		Handler:      mux,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	if *certFile != "" && *keyFile != "" {
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
		zap.String("addr", *listenAddr),
		zap.Bool("improved", *useImproved),
		zap.String("mongoTarget", os.Getenv("TARGET_MONGODB")))
	
	var err error
	if *certFile != "" && *keyFile != "" {
		err = srv.ListenAndServeTLS(*certFile, *keyFile)
	} else {
		logger.Warn("Running without TLS - not recommended for production")
		err = srv.ListenAndServe()
	}
	
	if err != nil && err != http.ErrServerClosed {
		logger.Fatal("Server failed", zap.Error(err))
	}
	
	logger.Info("Server stopped")
}