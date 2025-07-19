# GPG Password Store Viewer Makefile
# Author: Oleg Zubak <c4point@gmail.com>

.PHONY: help build install uninstall clean test lint format release

# Variables
BINARY_NAME = gpg_viewer
VERSION = 1.0.0
BUILD_DIR = build
INSTALL_DIR = /usr/local/bin
DESKTOP_DIR = /usr/share/applications
USER_DESKTOP_DIR = ~/.local/share/applications

# Go build flags
LDFLAGS = -ldflags "-X main.Version=$(VERSION) -X main.BuildTime=$(shell date -u '+%Y-%m-%d_%H:%M:%S')"

help: ## Show this help message
	@echo "GPG Password Store Viewer - Makefile"
	@echo ""
	@echo "Available targets:"
	@awk 'BEGIN {FS = ":.*?## "} /^[a-zA-Z_-]+:.*?## / {printf "  %-15s %s\n", $$1, $$2}' $(MAKEFILE_LIST)

build: ## Build the application
	@echo "Building $(BINARY_NAME)..."
	@mkdir -p $(BUILD_DIR)
	go build $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY_NAME)
	@chmod +x $(BUILD_DIR)/$(BINARY_NAME)
	@echo "Build complete: $(BUILD_DIR)/$(BINARY_NAME)"

build-debug: ## Build with debug information
	@echo "Building $(BINARY_NAME) with debug info..."
	@mkdir -p $(BUILD_DIR)
	go build $(LDFLAGS) -tags debug -o $(BUILD_DIR)/$(BINARY_NAME)
	@chmod +x $(BUILD_DIR)/$(BINARY_NAME)
	@echo "Debug build complete: $(BUILD_DIR)/$(BINARY_NAME)"

install: build ## Build and install system-wide
	@echo "Installing $(BINARY_NAME) system-wide..."
	@sudo cp $(BUILD_DIR)/$(BINARY_NAME) $(INSTALL_DIR)/
	@echo "[Desktop Entry]" | sudo tee $(DESKTOP_DIR)/gpg-viewer.desktop > /dev/null
	@echo "Name=GPG Password Store Viewer" | sudo tee -a $(DESKTOP_DIR)/gpg-viewer.desktop > /dev/null
	@echo "Comment=Modern GUI for password-store with GPG support" | sudo tee -a $(DESKTOP_DIR)/gpg-viewer.desktop > /dev/null
	@echo "Exec=/usr/local/bin/gpg_viewer" | sudo tee -a $(DESKTOP_DIR)/gpg-viewer.desktop > /dev/null
	@echo "Icon=security-high" | sudo tee -a $(DESKTOP_DIR)/gpg-viewer.desktop > /dev/null
	@echo "Terminal=false" | sudo tee -a $(DESKTOP_DIR)/gpg-viewer.desktop > /dev/null
	@echo "Type=Application" | sudo tee -a $(DESKTOP_DIR)/gpg-viewer.desktop > /dev/null
	@echo "Categories=Utility;Security;" | sudo tee -a $(DESKTOP_DIR)/gpg-viewer.desktop > /dev/null
	@echo "Keywords=password;gpg;security;" | sudo tee -a $(DESKTOP_DIR)/gpg-viewer.desktop > /dev/null
	@echo "Installation complete!"

install-user: build ## Build and install for current user only
	@echo "Installing $(BINARY_NAME) for current user..."
	@mkdir -p ~/.local/bin
	@cp $(BUILD_DIR)/$(BINARY_NAME) ~/.local/bin/
	@mkdir -p $(USER_DESKTOP_DIR)
	@echo "[Desktop Entry]" > $(USER_DESKTOP_DIR)/gpg-viewer.desktop
	@echo "Name=GPG Password Store Viewer" >> $(USER_DESKTOP_DIR)/gpg-viewer.desktop
	@echo "Comment=Modern GUI for password-store with GPG support" >> $(USER_DESKTOP_DIR)/gpg-viewer.desktop
	@echo "Exec=$$HOME/.local/bin/gpg_viewer" >> $(USER_DESKTOP_DIR)/gpg-viewer.desktop
	@echo "Icon=security-high" >> $(USER_DESKTOP_DIR)/gpg-viewer.desktop
	@echo "Terminal=false" >> $(USER_DESKTOP_DIR)/gpg-viewer.desktop
	@echo "Type=Application" >> $(USER_DESKTOP_DIR)/gpg-viewer.desktop
	@echo "Categories=Utility;Security;" >> $(USER_DESKTOP_DIR)/gpg-viewer.desktop
	@echo "Keywords=password;gpg;security;" >> $(USER_DESKTOP_DIR)/gpg-viewer.desktop
	@update-desktop-database $(USER_DESKTOP_DIR)
	@echo "User installation complete!"
	@echo "Add ~/.local/bin to your PATH if not already done:"
	@echo "export PATH=\"$$HOME/.local/bin:$$PATH\""

uninstall: ## Uninstall system-wide installation
	@echo "Uninstalling $(BINARY_NAME)..."
	@sudo rm -f $(INSTALL_DIR)/$(BINARY_NAME)
	@sudo rm -f $(DESKTOP_DIR)/gpg-viewer.desktop
	@echo "Uninstallation complete!"

uninstall-user: ## Uninstall user installation
	@echo "Uninstalling $(BINARY_NAME) for current user..."
	@rm -f ~/.local/bin/$(BINARY_NAME)
	@rm -f $(USER_DESKTOP_DIR)/gpg-viewer.desktop
	@update-desktop-database $(USER_DESKTOP_DIR)
	@echo "User uninstallation complete!"

clean: ## Clean build artifacts
	@echo "Cleaning build artifacts..."
	@rm -rf $(BUILD_DIR)
	@go clean
	@echo "Clean complete!"

test:
	@echo "Running tests..."
	go test ./...

test-verbose:
	@echo "Running tests with verbose output..."
	go test ./... -v

test-coverage:
	@echo "Running tests with coverage..."
	go test ./... -cover
	@echo "Generating coverage report..."
	go test ./... -coverprofile=coverage.out
	go tool cover -html=coverage.out -o coverage.html
	@echo "Coverage report generated: coverage.html"

test-benchmark:
	@echo "Running benchmarks..."
	go test ./... -bench=. -benchmem

lint: ## Run linter
	@echo "Running linter..."
	@if command -v golangci-lint >/dev/null 2>&1; then \
		golangci-lint run; \
	else \
		echo "golangci-lint not found. Install with: go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest"; \
	fi

format: ## Format code
	@echo "Formatting code..."
	go fmt ./...
	@if command -v goimports >/dev/null 2>&1; then \
		goimports -w .; \
	else \
		echo "goimports not found. Install with: go install golang.org/x/tools/cmd/goimports@latest"; \
	fi

deps: ## Download and tidy dependencies
	@echo "Downloading dependencies..."
	go mod download
	go mod tidy
	@echo "Dependencies updated!"

release: clean ## Build release binaries for multiple platforms
	@echo "Building release binaries..."
	@mkdir -p $(BUILD_DIR)/release
	
	# Linux
	@echo "Building for Linux..."
	GOOS=linux GOARCH=amd64 go build $(LDFLAGS) -o $(BUILD_DIR)/release/$(BINARY_NAME)_linux_amd64
	GOOS=linux GOARCH=arm64 go build $(LDFLAGS) -o $(BUILD_DIR)/release/$(BINARY_NAME)_linux_arm64
	
	# Windows
	@echo "Building for Windows..."
	GOOS=windows GOARCH=amd64 go build $(LDFLAGS) -o $(BUILD_DIR)/release/$(BINARY_NAME)_windows_amd64.exe
	GOOS=windows GOARCH=arm64 go build $(LDFLAGS) -o $(BUILD_DIR)/release/$(BINARY_NAME)_windows_arm64.exe
	
	# macOS
	@echo "Building for macOS..."
	GOOS=darwin GOARCH=amd64 go build $(LDFLAGS) -o $(BUILD_DIR)/release/$(BINARY_NAME)_darwin_amd64
	GOOS=darwin GOARCH=arm64 go build $(LDFLAGS) -o $(BUILD_DIR)/release/$(BINARY_NAME)_darwin_arm64
	
	@echo "Release builds complete in $(BUILD_DIR)/release/"

run: build ## Build and run the application
	@echo "Running $(BINARY_NAME)..."
	@$(BUILD_DIR)/$(BINARY_NAME)

dev: ## Run in development mode with hot reload (requires air)
	@if command -v air >/dev/null 2>&1; then \
		air; \
	else \
		echo "Air not found. Install with: go install github.com/cosmtrek/air@latest"; \
		echo "Or run: make run"; \
	fi

check: ## Check code quality
	@echo "Checking code quality..."
	@go vet ./...
	@go mod verify
	@echo "Code quality check complete!"

version: ## Show version information
	@echo "GPG Password Store Viewer v$(VERSION)"
	@echo "Go version: $(shell go version)"
	@echo "Build time: $(shell date -u '+%Y-%m-%d %H:%M:%S UTC')"

# Development helpers
setup-dev: deps ## Setup development environment
	@echo "Setting up development environment..."
	@go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest
	@go install golang.org/x/tools/cmd/goimports@latest
	@go install github.com/cosmtrek/air@latest
	@echo "Development environment setup complete!"

# Quick install for development
quick-install: build ## Quick install for development (user-local)
	@mkdir -p ~/.local/bin
	@cp $(BUILD_DIR)/$(BINARY_NAME) ~/.local/bin/
	@echo "Quick install complete! Run with: ~/.local/bin/$(BINARY_NAME)" 