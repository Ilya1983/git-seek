# git-seek Makefile
# Semantic search for git commits
#
# Usage:
#   make help       - Show available commands
#   make build      - Build the binary (downloads deps if needed)
#   make install    - Install to /usr/local (or PREFIX)
#   make test       - Run tests
#   make release    - Create release package

#==============================================================================
# Configuration
#==============================================================================

# Project metadata
PROJECT_NAME := git-seek
VERSION := $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
GIT_COMMIT := $(shell git rev-parse --short HEAD 2>/dev/null || echo "unknown")
BUILD_DATE := $(shell date -u +"%Y-%m-%dT%H:%M:%SZ")

# Go settings
GO := go
GOFLAGS := -trimpath
CGO_ENABLED := 1
LDFLAGS := -s -w -X main.version=$(VERSION) -X main.commit=$(GIT_COMMIT) -X main.date=$(BUILD_DATE)

# ONNX Runtime settings
ORT_VERSION := 1.23.2
ORT_BASE_URL := https://github.com/microsoft/onnxruntime/releases/download/v$(ORT_VERSION)

# SHA256 checksums for ONNX Runtime archives (optional security verification)
# To enable verification, set these to the actual checksums from the release
# Leave empty to skip verification (with warning)
ORT_SHA256_linux-x64 :=
ORT_SHA256_linux-aarch64 :=
ORT_SHA256_osx-x86_64 :=
ORT_SHA256_osx-arm64 :=

# Model settings (all-MiniLM-L6-v2 for embeddings)
MODEL_URL := https://media.githubusercontent.com/media/clems4ever/all-minilm-l6-v2-go/main/all_minilm_l6_v2/model.onnx
MODEL_FILE := patches/all-minilm-l6-v2-go/all_minilm_l6_v2/model.onnx
MODEL_SHA256 := 994a58868f7abacacbf2192aa0aae8f56da8c4505dbde2740c861b24426ede6b

# Directory structure
DEPS_DIR := deps
ORT_DIR := $(DEPS_DIR)/onnxruntime
DIST_DIR := dist

# Installation
PREFIX ?= /usr/local
INSTALL_BIN := $(PREFIX)/bin
INSTALL_LIB := $(PREFIX)/lib

# Linting
GOLANGCI_LINT_VERSION := v1.61.0

#==============================================================================
# Platform Detection
#==============================================================================

UNAME_S := $(shell uname -s 2>/dev/null || echo "Unknown")
UNAME_M := $(shell uname -m 2>/dev/null || echo "Unknown")

# Map uname output to ONNX Runtime naming conventions
ifeq ($(UNAME_S),Linux)
    HOST_OS := linux
    LIB_EXT := so
    LIB_PATH_VAR := LD_LIBRARY_PATH
    ifeq ($(UNAME_M),x86_64)
        HOST_ARCH := x64
        ORT_PLATFORM := linux-x64
    else ifeq ($(UNAME_M),aarch64)
        HOST_ARCH := aarch64
        ORT_PLATFORM := linux-aarch64
    else ifeq ($(UNAME_M),arm64)
        HOST_ARCH := aarch64
        ORT_PLATFORM := linux-aarch64
    endif
else ifeq ($(UNAME_S),Darwin)
    HOST_OS := darwin
    LIB_EXT := dylib
    LIB_PATH_VAR := DYLD_LIBRARY_PATH
    ifeq ($(UNAME_M),x86_64)
        HOST_ARCH := x86_64
        ORT_PLATFORM := osx-x86_64
    else ifeq ($(UNAME_M),arm64)
        HOST_ARCH := arm64
        ORT_PLATFORM := osx-arm64
    endif
endif

# Validate platform detection
ifndef ORT_PLATFORM
    $(error Unsupported platform: $(UNAME_S) $(UNAME_M). Supported: Linux x86_64/aarch64, Darwin x86_64/arm64)
endif

# ONNX Runtime paths
ORT_DOWNLOAD_DIR := $(ORT_DIR)/$(ORT_PLATFORM)
ORT_LIB_DIR := $(ORT_DOWNLOAD_DIR)/lib
ORT_LIB_NAME := libonnxruntime.$(LIB_EXT)
ORT_LIB_FILE := $(ORT_LIB_DIR)/$(ORT_LIB_NAME)

# ONNX Runtime download URL
ORT_ARCHIVE := onnxruntime-$(ORT_PLATFORM)-$(ORT_VERSION).tgz
ORT_URL := $(ORT_BASE_URL)/$(ORT_ARCHIVE)

# Build output
BINARY := $(PROJECT_NAME)

#==============================================================================
# Default Target
#==============================================================================

.PHONY: all
all: build

#==============================================================================
# Help
#==============================================================================

.PHONY: help
help: ## Show this help message
	@echo "git-seek - Semantic search for git commits"
	@echo ""
	@echo "Usage: make [target]"
	@echo ""
	@echo "Build targets:"
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | \
		grep -E '(build|setup|install|clean)' | sort | \
		awk 'BEGIN {FS = ":.*?## "}; {printf "  \033[36m%-18s\033[0m %s\n", $$1, $$2}'
	@echo ""
	@echo "Test targets:"
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | \
		grep -E '(test|lint|fmt|vet)' | sort | \
		awk 'BEGIN {FS = ":.*?## "}; {printf "  \033[36m%-18s\033[0m %s\n", $$1, $$2}'
	@echo ""
	@echo "Release targets:"
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | \
		grep -E '(release)' | sort | \
		awk 'BEGIN {FS = ":.*?## "}; {printf "  \033[36m%-18s\033[0m %s\n", $$1, $$2}'
	@echo ""
	@echo "Environment:"
	@echo "  Platform:      $(HOST_OS)-$(HOST_ARCH)"
	@echo "  ORT Platform:  $(ORT_PLATFORM)"
	@echo "  ORT Version:   $(ORT_VERSION)"
	@echo "  Go Version:    $(shell $(GO) version 2>/dev/null | cut -d' ' -f3 || echo 'not found')"
	@echo ""
	@echo "Variables (override with VAR=value):"
	@echo "  PREFIX         Installation prefix (default: /usr/local)"
	@echo "  ORT_VERSION    ONNX Runtime version (default: $(ORT_VERSION))"
	@echo "  VERSION        Release version (default: git describe)"

#==============================================================================
# Dependencies
#==============================================================================

# Create directories
$(DEPS_DIR):
	@mkdir -p $@

$(ORT_DIR): | $(DEPS_DIR)
	@mkdir -p $@

$(DIST_DIR):
	@mkdir -p $@

# Get checksum for current platform
ORT_SHA256 := $(ORT_SHA256_$(ORT_PLATFORM))

# Download and extract ONNX Runtime
$(ORT_LIB_FILE): | $(ORT_DIR)
	@echo "Downloading ONNX Runtime $(ORT_VERSION) for $(ORT_PLATFORM)..."
	@mkdir -p $(ORT_DOWNLOAD_DIR)
	@curl -fsSL --progress-bar -o $(ORT_DIR)/$(ORT_ARCHIVE) $(ORT_URL)
ifneq ($(ORT_SHA256),)
	@echo "Verifying checksum..."
	@echo "$(ORT_SHA256)  $(ORT_DIR)/$(ORT_ARCHIVE)" | sha256sum -c - || \
		(echo "ERROR: Checksum verification failed!" && rm -f $(ORT_DIR)/$(ORT_ARCHIVE) && exit 1)
else
	@echo "WARNING: Checksum verification skipped (no checksum configured)"
endif
	@tar -xzf $(ORT_DIR)/$(ORT_ARCHIVE) -C $(ORT_DIR)
	@mv $(ORT_DIR)/onnxruntime-$(ORT_PLATFORM)-$(ORT_VERSION)/* $(ORT_DOWNLOAD_DIR)/
	@rm -rf $(ORT_DIR)/onnxruntime-$(ORT_PLATFORM)-$(ORT_VERSION)
	@rm -f $(ORT_DIR)/$(ORT_ARCHIVE)
	@echo "ONNX Runtime installed to $(ORT_DOWNLOAD_DIR)"

# Download embedding model (required for semantic search)
$(MODEL_FILE):
	@echo "Downloading all-MiniLM-L6-v2 model (~90MB)..."
	@mkdir -p $(dir $(MODEL_FILE))
	@curl -fsSL --progress-bar -o $(MODEL_FILE) $(MODEL_URL)
	@echo "Verifying model checksum..."
	@echo "$(MODEL_SHA256)  $(MODEL_FILE)" | sha256sum -c - || (rm -f $(MODEL_FILE) && echo "ERROR: Model checksum verification failed!" && exit 1)
	@echo "Model downloaded and verified: $(MODEL_FILE)"

.PHONY: setup check-deps
setup: $(ORT_LIB_FILE) $(MODEL_FILE) ## Download ONNX Runtime and model for current platform
	@echo ""
	@echo "Setup complete!"
	@echo "ONNX Runtime $(ORT_VERSION) for $(ORT_PLATFORM) is ready."

check-deps: ## Check if dependencies are installed
	@if [ ! -f "$(ORT_LIB_FILE)" ]; then \
		echo "Error: ONNX Runtime not found at $(ORT_LIB_FILE)"; \
		echo "Run 'make setup' to download dependencies"; \
		exit 1; \
	fi
	@if [ ! -f "$(MODEL_FILE)" ]; then \
		echo "Error: Model not found at $(MODEL_FILE)"; \
		echo "Run 'make setup' to download dependencies"; \
		exit 1; \
	fi
	@echo "Dependencies OK"
	@echo "  ONNX Runtime: $(ORT_VERSION) ($(ORT_PLATFORM))"
	@echo "  Model: all-MiniLM-L6-v2"

#==============================================================================
# Build
#==============================================================================

.PHONY: build build-only
build: $(ORT_LIB_FILE) $(MODEL_FILE) build-only ## Build the binary (downloads deps if needed)

build-only: ## Build the binary (assumes deps exist)
	@echo "Building $(PROJECT_NAME) $(VERSION)..."
	@CGO_ENABLED=$(CGO_ENABLED) \
		$(LIB_PATH_VAR)=$(shell pwd)/$(ORT_LIB_DIR) \
		ONNXRUNTIME_LIB_PATH=$(shell pwd)/$(ORT_LIB_FILE) \
		$(GO) build $(GOFLAGS) -ldflags "$(LDFLAGS)" -o $(BINARY) .
	@echo ""
	@echo "Build successful: ./$(BINARY)"
	@echo ""
	@echo "To run, set the library path:"
ifeq ($(HOST_OS),linux)
	@echo "  export LD_LIBRARY_PATH=$(shell pwd)/$(ORT_LIB_DIR)"
	@echo "  export ONNXRUNTIME_LIB_PATH=$(shell pwd)/$(ORT_LIB_FILE)"
else ifeq ($(HOST_OS),darwin)
	@echo "  export DYLD_LIBRARY_PATH=$(shell pwd)/$(ORT_LIB_DIR)"
	@echo "  export ONNXRUNTIME_LIB_PATH=$(shell pwd)/$(ORT_LIB_FILE)"
endif
	@echo ""
	@echo "Then run:"
	@echo "  ./$(BINARY) --help"

#==============================================================================
# Install / Uninstall
#==============================================================================

.PHONY: install uninstall
install: build ## Install to PREFIX (default: /usr/local)
	@echo "Installing to $(PREFIX)..."
	@install -d $(INSTALL_BIN)
	@install -d $(INSTALL_LIB)
	@install -m 755 $(BINARY) $(INSTALL_BIN)/
	@cp -P $(ORT_LIB_DIR)/$(ORT_LIB_NAME)* $(INSTALL_LIB)/ 2>/dev/null || \
		cp $(ORT_LIB_DIR)/$(ORT_LIB_NAME) $(INSTALL_LIB)/
	@echo ""
	@echo "Installed:"
	@echo "  Binary: $(INSTALL_BIN)/$(PROJECT_NAME)"
	@echo "  Library: $(INSTALL_LIB)/$(ORT_LIB_NAME)"
	@echo ""
	@echo "Add to your shell profile:"
	@echo "  export ONNXRUNTIME_LIB_PATH=$(INSTALL_LIB)/$(ORT_LIB_NAME)"
ifeq ($(HOST_OS),linux)
	@echo ""
	@echo "You may also need to run: sudo ldconfig"
endif

uninstall: ## Remove installed files
	@echo "Uninstalling from $(PREFIX)..."
	@rm -f $(INSTALL_BIN)/$(PROJECT_NAME)
	@rm -f $(INSTALL_LIB)/libonnxruntime.*
	@echo "Uninstalled."

#==============================================================================
# Test
#==============================================================================

.PHONY: test test-short test-race test-coverage
test: $(ORT_LIB_FILE) ## Run all tests
	@echo "Running tests..."
	@CGO_ENABLED=$(CGO_ENABLED) \
		$(LIB_PATH_VAR)=$(shell pwd)/$(ORT_LIB_DIR) \
		ONNXRUNTIME_LIB_PATH=$(shell pwd)/$(ORT_LIB_FILE) \
		$(GO) test -v ./...

test-short: $(ORT_LIB_FILE) ## Run tests (short mode)
	@CGO_ENABLED=$(CGO_ENABLED) \
		$(LIB_PATH_VAR)=$(shell pwd)/$(ORT_LIB_DIR) \
		ONNXRUNTIME_LIB_PATH=$(shell pwd)/$(ORT_LIB_FILE) \
		$(GO) test -short ./...

test-race: $(ORT_LIB_FILE) ## Run tests with race detector
	@CGO_ENABLED=$(CGO_ENABLED) \
		$(LIB_PATH_VAR)=$(shell pwd)/$(ORT_LIB_DIR) \
		ONNXRUNTIME_LIB_PATH=$(shell pwd)/$(ORT_LIB_FILE) \
		$(GO) test -race ./...

test-coverage: $(ORT_LIB_FILE) ## Run tests with coverage report
	@CGO_ENABLED=$(CGO_ENABLED) \
		$(LIB_PATH_VAR)=$(shell pwd)/$(ORT_LIB_DIR) \
		ONNXRUNTIME_LIB_PATH=$(shell pwd)/$(ORT_LIB_FILE) \
		$(GO) test -coverprofile=coverage.out ./...
	@$(GO) tool cover -html=coverage.out -o coverage.html
	@echo "Coverage report: coverage.html"

#==============================================================================
# Lint / Format
#==============================================================================

.PHONY: lint lint-install fmt vet
lint: ## Run golangci-lint
	@if ! command -v golangci-lint >/dev/null 2>&1; then \
		echo "Error: golangci-lint not found"; \
		echo "Run 'make lint-install' to install it"; \
		exit 1; \
	fi
	@golangci-lint run ./...

lint-install: ## Install golangci-lint
	@echo "Installing golangci-lint $(GOLANGCI_LINT_VERSION)..."
	@curl -sSfL https://raw.githubusercontent.com/golangci/golangci-lint/master/install.sh | \
		sh -s -- -b $(shell $(GO) env GOPATH)/bin $(GOLANGCI_LINT_VERSION)
	@echo "Installed golangci-lint to $(shell $(GO) env GOPATH)/bin"

fmt: ## Format code with goimports
	@if ! command -v goimports >/dev/null 2>&1; then \
		echo "Installing goimports..."; \
		$(GO) install golang.org/x/tools/cmd/goimports@latest; \
	fi
	@goimports -w .
	@echo "Code formatted."

vet: ## Run go vet
	@$(GO) vet ./...

#==============================================================================
# Clean
#==============================================================================

.PHONY: clean clean-deps clean-all
clean: ## Remove build artifacts
	@rm -f $(BINARY)
	@rm -rf $(DIST_DIR)
	@rm -f coverage.out coverage.html
	@echo "Cleaned build artifacts."

clean-deps: ## Remove downloaded dependencies
	@rm -rf $(DEPS_DIR)
	@echo "Cleaned dependencies."

clean-all: clean clean-deps ## Remove all generated files
	@echo "Cleaned everything."

#==============================================================================
# Release
#==============================================================================

RELEASE_NAME := $(PROJECT_NAME)-$(ORT_PLATFORM)-$(VERSION)
RELEASE_DIR := $(DIST_DIR)/$(RELEASE_NAME)

.PHONY: release-local release release-checksums
release-local: build | $(DIST_DIR) ## Create release package for current platform
	@echo "Creating release package: $(RELEASE_NAME)..."
	@rm -rf $(RELEASE_DIR)
	@mkdir -p $(RELEASE_DIR)/lib
	@cp $(BINARY) $(RELEASE_DIR)/
	@cp -P $(ORT_LIB_DIR)/$(ORT_LIB_NAME)* $(RELEASE_DIR)/lib/ 2>/dev/null || \
		cp $(ORT_LIB_DIR)/$(ORT_LIB_NAME) $(RELEASE_DIR)/lib/
	@if [ -f LICENSE ]; then cp LICENSE $(RELEASE_DIR)/; fi
	@echo "# $(PROJECT_NAME) $(VERSION)" > $(RELEASE_DIR)/README.md
	@echo "" >> $(RELEASE_DIR)/README.md
	@echo "Semantic search for git commits." >> $(RELEASE_DIR)/README.md
	@echo "" >> $(RELEASE_DIR)/README.md
	@echo "## Quick Start" >> $(RELEASE_DIR)/README.md
	@echo "" >> $(RELEASE_DIR)/README.md
	@echo "Run these commands from the directory where you extracted the archive:" >> $(RELEASE_DIR)/README.md
	@echo "" >> $(RELEASE_DIR)/README.md
	@echo "\`\`\`bash" >> $(RELEASE_DIR)/README.md
ifeq ($(HOST_OS),linux)
	@echo "export LD_LIBRARY_PATH=\$$(pwd)/lib:\$$LD_LIBRARY_PATH" >> $(RELEASE_DIR)/README.md
	@echo "export ONNXRUNTIME_LIB_PATH=\$$(pwd)/lib/libonnxruntime.so" >> $(RELEASE_DIR)/README.md
else ifeq ($(HOST_OS),darwin)
	@echo "export DYLD_LIBRARY_PATH=\$$(pwd)/lib:\$$DYLD_LIBRARY_PATH" >> $(RELEASE_DIR)/README.md
	@echo "export ONNXRUNTIME_LIB_PATH=\$$(pwd)/lib/libonnxruntime.dylib" >> $(RELEASE_DIR)/README.md
endif
	@echo "./$(PROJECT_NAME) --help" >> $(RELEASE_DIR)/README.md
	@echo "\`\`\`" >> $(RELEASE_DIR)/README.md
	@echo "" >> $(RELEASE_DIR)/README.md
	@echo "**Note:** If you move the binary to another location, update the paths accordingly" >> $(RELEASE_DIR)/README.md
	@echo "or install to a system directory (e.g., copy lib/* to /usr/local/lib)." >> $(RELEASE_DIR)/README.md
	@echo "" >> $(RELEASE_DIR)/README.md
	@echo "**To make permanent:** Add the export commands to your shell profile (~/.bashrc or ~/.zshrc)." >> $(RELEASE_DIR)/README.md
	@echo "" >> $(RELEASE_DIR)/README.md
	@echo "## Usage" >> $(RELEASE_DIR)/README.md
	@echo "" >> $(RELEASE_DIR)/README.md
	@echo "\`\`\`bash" >> $(RELEASE_DIR)/README.md
	@echo "# Index your repository" >> $(RELEASE_DIR)/README.md
	@echo "./$(PROJECT_NAME) --index" >> $(RELEASE_DIR)/README.md
	@echo "" >> $(RELEASE_DIR)/README.md
	@echo "# Search commits" >> $(RELEASE_DIR)/README.md
	@echo "./$(PROJECT_NAME) \"your search query\"" >> $(RELEASE_DIR)/README.md
	@echo "\`\`\`" >> $(RELEASE_DIR)/README.md
	@cd $(DIST_DIR) && tar -czf $(RELEASE_NAME).tar.gz $(RELEASE_NAME)
	@rm -rf $(RELEASE_DIR)
	@echo ""
	@echo "Release package created:"
	@echo "  $(DIST_DIR)/$(RELEASE_NAME).tar.gz"

release-checksums: ## Generate checksums for release files
	@echo "Generating checksums..."
	@cd $(DIST_DIR) && \
		if command -v sha256sum >/dev/null 2>&1; then \
			sha256sum *.tar.gz *.zip 2>/dev/null > checksums.txt || true; \
		else \
			shasum -a 256 *.tar.gz *.zip 2>/dev/null > checksums.txt || true; \
		fi
	@if [ -s $(DIST_DIR)/checksums.txt ]; then \
		echo "Checksums:"; \
		cat $(DIST_DIR)/checksums.txt; \
	else \
		echo "No release files found to checksum."; \
	fi

release: release-local release-checksums ## Create release with checksums

#==============================================================================
# Utility
#==============================================================================

.PHONY: run
run: build ## Build and run with sample query
	@$(LIB_PATH_VAR)=$(shell pwd)/$(ORT_LIB_DIR) \
		ONNXRUNTIME_LIB_PATH=$(shell pwd)/$(ORT_LIB_FILE) \
		./$(BINARY) --help

.PHONY: info
info: ## Show build information
	@echo "Project:       $(PROJECT_NAME)"
	@echo "Version:       $(VERSION)"
	@echo "Git Commit:    $(GIT_COMMIT)"
	@echo "Build Date:    $(BUILD_DATE)"
	@echo "Go Version:    $(shell $(GO) version | cut -d' ' -f3)"
	@echo "Platform:      $(HOST_OS)/$(HOST_ARCH)"
	@echo "ORT Platform:  $(ORT_PLATFORM)"
	@echo "ORT Version:   $(ORT_VERSION)"
	@echo "ORT Library:   $(ORT_LIB_FILE)"
	@echo "Binary:        $(BINARY)"
