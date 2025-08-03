# Secure Air-Gapped Tunnel System
# Build automation and common tasks

.PHONY: help build build-linux build-server build-client clean dev up down logs test deps

# Default target
help: ## Show available commands
	@echo "Available commands:"
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | sort | awk 'BEGIN {FS = ":.*?## "}; {printf "  \033[36m%-15s\033[0m %s\n", $$1, $$2}'

# Build targets
build: build-server build-client ## Build both server and client binaries

build-linux: ## Build Linux binaries (production)
	@echo "Building Linux binaries..."
	GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build -ldflags="-s -w" -trimpath -o bin/tunnel-server-linux server/main.go
	GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build -ldflags="-s -w" -trimpath -o bin/tunnel-client-linux client/main.go
	@echo "✅ Linux binaries built successfully"

build-server: ## Build tunnel server
	@echo "Building tunnel server..."
	go build -o bin/tunnel-server server/main.go
	@echo "✅ Server binary built: bin/tunnel-server"

build-client: ## Build tunnel client  
	@echo "Building tunnel client..."
	go build -o bin/tunnel-client client/main.go
	@echo "✅ Client binary built: bin/tunnel-client"

# Development targets
dev: up ## Start development environment

up: ## Start Docker Compose services
	@echo "Starting development environment..."
	docker-compose up -d
	@echo "✅ Services started. Access web app at http://localhost:8080"

down: ## Stop Docker Compose services
	@echo "Stopping development environment..."
	docker-compose down
	@echo "✅ Services stopped"

logs: ## View Docker Compose logs
	docker-compose logs -f

# Testing
test: ## Test tunnel connection
	@echo "Testing tunnel connection..."
	@if docker-compose ps | grep -q "tunnel-server.*Up"; then \
		echo "✅ Tunnel server is running"; \
		curl -f http://localhost:8443/health || echo "❌ Health check failed"; \
	else \
		echo "❌ Tunnel server is not running. Run 'make up' first"; \
	fi

# Daemon management
install-client-daemon: build-linux ## Build and install tunnel client as systemd daemon
	@echo "Installing tunnel client daemon..."
	sudo ./daemon/client/install-daemon.sh
	@echo "✅ Client daemon installed. Configure /etc/tunnel-client/config and start with: sudo systemctl start tunnel-client"

install-server-daemon: build-linux ## Build and install tunnel server as systemd daemon
	@echo "Installing tunnel server daemon..."
	sudo ./daemon/server/install-server-daemon.sh
	@echo "✅ Server daemon installed. Configure /etc/tunnel-server/config and start with: sudo systemctl start tunnel-server"

# Legacy alias for compatibility
install-daemon: install-client-daemon ## Alias for install-client-daemon (for backward compatibility)

# Kubernetes deployment
k8s-deploy: ## Deploy to Kubernetes
	@echo "Deploying to Kubernetes..."
	kubectl apply -k k8s/
	@echo "✅ Deployed to Kubernetes. Check status with: kubectl get pods -n tunnel-system"

k8s-status: ## Check Kubernetes deployment status
	kubectl get pods -n tunnel-system -l app.kubernetes.io/component=tunnel-client

k8s-logs: ## View Kubernetes logs
	kubectl logs -n tunnel-system -l app.kubernetes.io/name=tunnel-client-k8s-api -f

# Maintenance
clean: ## Clean build artifacts
	@echo "Cleaning build artifacts..."
	rm -f bin/tunnel-server bin/tunnel-client
	rm -f bin/tunnel-server-linux bin/tunnel-client-linux
	docker-compose down --remove-orphans 2>/dev/null || true
	@echo "✅ Clean complete"

deps: ## Download Go dependencies
	@echo "Downloading dependencies..."
	go mod download
	go mod tidy
	@echo "✅ Dependencies updated"

# Security
certs: ## Generate test TLS certificates
	@echo "Generating test certificates..."
	./scripts/generate-certs.sh localhost
	chmod 644 certs/*
	@echo "✅ Test certificates generated in certs/ with Docker-friendly permissions"

certs-domain: ## Generate certificates for specific domain (usage: make certs-domain DOMAIN=your-server.com)
	@if [ -z "$(DOMAIN)" ]; then \
		echo "❌ Error: DOMAIN variable required"; \
		echo "Usage: make certs-domain DOMAIN=your-server.com"; \
		exit 1; \
	fi
	@echo "Generating certificates for $(DOMAIN)..."
	./scripts/generate-certs.sh $(DOMAIN)
	chmod 644 certs/*
	@echo "✅ Certificates generated for $(DOMAIN) with Docker-friendly permissions"

letsencrypt: ## Setup Let's Encrypt certificate (usage: make letsencrypt DOMAIN=your-server.com)
	@if [ -z "$(DOMAIN)" ]; then \
		echo "❌ Error: DOMAIN variable required"; \
		echo "Usage: make letsencrypt DOMAIN=your-server.com [EMAIL=admin@domain.com]"; \
		exit 1; \
	fi
	@echo "Setting up Let's Encrypt for $(DOMAIN)..."
	./scripts/setup-letsencrypt.sh $(DOMAIN) $(EMAIL)
	@echo "✅ Let's Encrypt certificate configured"

# Docker images
docker-build: ## Build Docker images
	@echo "Building Docker images..."
	docker build -f Dockerfile.server -t tunnel-server:latest .
	docker build -f Dockerfile.client -t tunnel-client:latest .
	@echo "✅ Docker images built"

# Release management
release: ## Build release assets (usage: make release VERSION=v1.2.0)
	@if [ -z "$(VERSION)" ]; then \
		echo "❌ Error: VERSION variable required"; \
		echo "Usage: make release VERSION=v1.2.0"; \
		exit 1; \
	fi
	@echo "Building release $(VERSION)..."
	./scripts/build-release.sh $(VERSION)
	@echo "✅ Release $(VERSION) built successfully"

github-release: ## Create GitHub release (requires gh CLI)
	@if [ ! -d "release" ]; then \
		echo "❌ Error: No release directory found. Run 'make release VERSION=vX.X.X' first"; \
		exit 1; \
	fi
	@if ! command -v gh &> /dev/null; then \
		echo "❌ Error: GitHub CLI (gh) not found. Install with: brew install gh"; \
		exit 1; \
	fi
	@echo "Creating GitHub release..."
	@VERSION=$$(ls release/tunnel-*.tar.gz | head -1 | sed -E 's/.*tunnel-(.+)-linux-amd64.tar.gz/\1/'); \
	git add . && git commit -m "Release $$VERSION" && git push && \
	gh release create $$VERSION release/* --title "Tunnel System $$VERSION" --notes-file release/RELEASE_NOTES.md
	@echo "✅ GitHub release created successfully"

# Project info
info: ## Show project information
	@echo "📁 Project Structure:"
	@echo "  bin/                    - Compiled binaries"
	@echo "  client/main.go          - Client entry point"
	@echo "  server/main.go          - Server entry point"
	@echo "  pkg/tunnel/             - Core tunnel logic"
	@echo "  daemon/client/          - Client daemon installation"
	@echo "  daemon/server/          - Server daemon installation"
	@echo "  k8s/                    - Kubernetes deployment"
	@echo "  docker-compose.yml      - Docker development setup"
	@echo ""
	@echo "🚀 Quick Commands:"
	@echo "  make build-linux        - Build production binaries"
	@echo "  make install-client-daemon - Install client as Linux daemon"
	@echo "  make install-server-daemon - Install server as Linux daemon"
	@echo "  make release VERSION=v1.0.0 - Build release assets"
	@echo "  make github-release     - Create GitHub release"
	@echo "  make k8s-deploy         - Deploy to Kubernetes"
	@echo "  make dev                - Start development environment"