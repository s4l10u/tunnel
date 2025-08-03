package main

import (
	"context"
	"crypto/tls"
	"flag"
	"log"
	"net/http"
	"os"
	"os/signal"
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
		
		// Start TCP forwarding listeners with error handling
		if err := server.StartTCPForwarder(8080, "airgap-web"); err != nil {
			logger.Error("Failed to start web forwarder", zap.Error(err))
		} else {
			logger.Info("Started TCP forwarder", zap.Int("port", 8080), zap.String("service", "web"))
		}
		
		if err := server.StartTCPForwarder(5432, "airgap-db"); err != nil {
			logger.Error("Failed to start database forwarder", zap.Error(err))
		} else {
			logger.Info("Started TCP forwarder", zap.Int("port", 5432), zap.String("service", "database"))
		}
		
		if err := server.StartTCPForwarder(2222, "airgap-ssh"); err != nil {
			logger.Error("Failed to start SSH forwarder", zap.Error(err))
		} else {
			logger.Info("Started TCP forwarder", zap.Int("port", 2222), zap.String("service", "ssh"))
		}
		
		if err := server.StartTCPForwarder(27017, "airgap-mongodb"); err != nil {
			logger.Error("Failed to start MongoDB forwarder", zap.Error(err))
		} else {
			logger.Info("Started TCP forwarder", zap.Int("port", 27017), zap.String("service", "mongodb"))
		}
		
		if err := server.StartTCPForwarder(6443, "airgap-k8s-api"); err != nil {
			logger.Error("Failed to start Kubernetes API server forwarder", zap.Error(err))
		} else {
			logger.Info("Started TCP forwarder", zap.Int("port", 6443), zap.String("service", "kubernetes-api"))
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