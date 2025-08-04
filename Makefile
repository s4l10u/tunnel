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

up: ## Start Docker Compose services with new YAML config
	@echo "🚀 Starting development environment with YAML configuration..."
	@if [ ! -f .env ]; then \
		echo "📋 Creating .env file from template..."; \
		cp .env.example .env; \
	fi
	docker-compose up -d
	@echo "✅ Services started with YAML configuration!"
	@echo "   📱 Web app: http://localhost:8080"
	@echo "   🗄️  Database: localhost:5432 (user: airgapped, pass: airgapped-password)"
	@echo "   🔐 SSH: ssh airgapped@localhost -p 2222 (pass: airgapped)"
	@echo "   📊 Health: http://localhost:8443/health"
	@echo "   📋 Config: ./docker-config.yaml"

up-full: ## Start all services including optional ones (Redis, Elasticsearch)
	@echo "🚀 Starting full development environment with optional services..."
	@if [ ! -f .env ]; then \
		echo "📋 Creating .env file from template..."; \
		cp .env.example .env; \
	fi
	docker-compose --profile optional up -d
	@echo "✅ All services started including Redis and Elasticsearch!"
	@echo "   📱 Web app: http://localhost:8080"
	@echo "   🗄️  Database: localhost:5432"
	@echo "   🔐 SSH: ssh airgapped@localhost -p 2222"
	@echo "   🔴 Redis: localhost:6379 (pass: airgapped-redis-password)"
	@echo "   🔍 Elasticsearch: http://localhost:9200"
	@echo "   💡 Enable tunnels: ENABLE_REDIS=true ENABLE_ELASTICSEARCH=true make up-full"

down: ## Stop Docker Compose services
	@echo "Stopping development environment..."
	docker-compose down
	@echo "✅ Services stopped"

logs: ## View Docker Compose logs
	docker-compose logs -f

# Testing
test: ## Test tunnel connection and YAML configuration
	@echo "🗺️ Testing tunnel connection and YAML configuration..."
	@if docker-compose ps | grep -q "tunnel-server.*Up"; then \
		echo "✅ Tunnel server is running"; \
		echo "📊 Testing health endpoint..."; \
		curl -f http://localhost:8443/health | jq . || echo "❌ Health check failed"; \
		echo "🌐 Testing web tunnel..."; \
		curl -f http://localhost:8080 > /dev/null && echo "✅ Web tunnel working" || echo "❌ Web tunnel failed"; \
		echo "📊 Testing database tunnel..."; \
		nc -z localhost 5432 && echo "✅ Database tunnel working" || echo "❌ Database tunnel failed"; \
	else \
		echo "❌ Tunnel server is not running. Run 'make up' first"; \
	fi

test-yaml: ## Test YAML configuration loading
	@echo "📋 Testing YAML configuration..."
	@if [ -f docker-config.yaml ]; then \
		echo "✅ YAML config found: docker-config.yaml"; \
		docker run --rm -v "$$(pwd)/docker-config.yaml:/config.yaml" -e TUNNEL_TOKEN=test mikefarah/yq eval '.server.token' /config.yaml || echo "Valid YAML structure"; \
	else \
		echo "❌ YAML config not found"; \
	fi

test-full: test test-yaml ## Run all tests including YAML configuration

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
	@echo "🚀 Building release $(VERSION)..."
	./scripts/build-release.sh $(VERSION)
	@echo "✅ Release $(VERSION) built successfully"

validate-release: ## Validate release artifacts
	@if [ ! -d "artifacts" ]; then \
		echo "❌ Error: No artifacts directory found. Run 'make release VERSION=vX.X.X' first"; \
		exit 1; \
	fi
	@echo "🔍 Validating release artifacts..."
	@echo "📋 Files in artifacts/:"
	@ls -la artifacts/
	@echo "📋 Required files check:"
	@for file in daemon.tar.gz install-client.sh install-server.sh checksums.txt RELEASE_NOTES.md; do \
		if [ -f "artifacts/$$file" ]; then \
			echo "✅ $$file"; \
		else \
			echo "❌ $$file (missing)"; \
		fi; \
	done
	@echo "📋 Platform binaries check:"
	@for platform in linux-amd64 linux-arm64 linux-386 darwin-amd64 darwin-arm64 windows-amd64 windows-386; do \
		if [[ "$$platform" == *"windows"* ]]; then \
			ext="zip"; \
		else \
			ext="tar.gz"; \
		fi; \
		version=$$(ls artifacts/tunnel-*-linux-amd64.tar.gz | head -1 | sed -E 's/.*tunnel-(.+)-linux-amd64.tar.gz/\1/' 2>/dev/null || echo "unknown"); \
		file="tunnel-$$version-$$platform.$$ext"; \
		if [ -f "artifacts/$$file" ]; then \
			echo "✅ $$file"; \
		else \
			echo "❌ $$file (missing)"; \
		fi; \
	done
	@echo "✅ Validation complete"

github-release: ## Create GitHub release (requires gh CLI)
	@if [ ! -d "artifacts" ]; then \
		echo "❌ Error: No artifacts directory found. Run 'make release VERSION=vX.X.X' first"; \
		exit 1; \
	fi
	@if ! command -v gh &> /dev/null; then \
		echo "❌ Error: GitHub CLI (gh) not found. Install with: brew install gh"; \
		exit 1; \
	fi
	@echo "🚀 Creating GitHub release..."
	@VERSION=$$(ls artifacts/tunnel-*-linux-amd64.tar.gz | head -1 | sed -E 's/.*tunnel-(.+)-linux-amd64.tar.gz/\1/'); \
	echo "📋 Detected version: $$VERSION"; \
	echo "📦 Validating artifacts..."; \
	ls -la artifacts/; \
	echo "📤 Creating release..."; \
	if git tag -l | grep -q "$$VERSION"; then \
		echo "🏷️  Tag $$VERSION already exists"; \
	else \
		echo "🏷️  Creating tag $$VERSION"; \
		git tag -a "$$VERSION" -m "Release $$VERSION"; \
		git push origin "$$VERSION"; \
	fi; \
	if gh release view "$$VERSION" >/dev/null 2>&1; then \
		echo "📦 Release $$VERSION already exists, uploading additional assets..."; \
		gh release upload "$$VERSION" artifacts/* --clobber; \
	else \
		echo "📦 Creating new release $$VERSION..."; \
		gh release create "$$VERSION" artifacts/* --title "🎆 Tunnel System $$VERSION" --notes-file artifacts/RELEASE_NOTES.md; \
	fi
	@echo "✅ GitHub release completed successfully"
	@echo "📋 Release URL: https://github.com/$$(git config --get remote.origin.url | sed 's/.*github.com[:\/]\(.*\)\.git/\1/')/releases/latest"

quick-release: ## Build and release in one command (usage: make quick-release VERSION=v1.2.1)
	@if [ -z "$(VERSION)" ]; then \
		echo "❌ Error: VERSION variable required"; \
		echo "Usage: make quick-release VERSION=v1.2.1"; \
		exit 1; \
	fi
	@echo "🚀 Quick release $(VERSION) - building and publishing..."
	make release VERSION=$(VERSION)
	make validate-release
	make github-release
	@echo "🎉 Quick release $(VERSION) completed!"
	@echo "📋 Test installation:"
	@echo "  curl -fsSL https://github.com/$$(git config --get remote.origin.url | sed 's/.*github.com[:\/]\(.*\)\.git/\1/')/releases/latest/download/install-server.sh | sudo bash"

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