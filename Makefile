.PHONY: help test coverage lint build clean ci badge tag tag-create tag-push tag-delete tag-minor tag-major tag-purge format mod-tidy

GO        ?= go
COVERMODE ?= atomic

BADGE_DIR   ?= .public
BADGE_LABEL ?= coverage

COVERPROFILE_ALL ?= coverage.out

# Automatically discover modules
MODULES := core validator controller pipes guards interceptors exceptions swagger di platform
MODULE_PACKAGES := ./core/... ./validator/... ./controller/... ./pipes/... ./guards/... ./interceptors/... ./exceptions/... ./swagger/... ./di/... ./platform/...
GOPATH_BIN := $(shell $(GO) env GOPATH)/bin

# Path to testing tools
TOOLS_PKG := github.com/gonest-dev/gonest-tools
LINT_TOOL := github.com/golangci/golangci-lint/v2/cmd/golangci-lint@v2.11.1

# Bypass Go proxy for private/recently public tools
export GOPRIVATE := github.com/gonest-dev/*

.DEFAULT_GOAL := help

help: ## Show this help
	@echo "GoNest Framework - Available Commands"
	@echo ""
	@echo "Testing:"
	@echo "  make test                 # Run tests on all modules"
	@echo "  make coverage             # Generate coverage profile"
	@echo "  make badge                # Generate coverage badge"
	@echo "  make ci                   # Run CI (test + coverage + badge)"
	@echo ""
	@echo "Building:"
	@echo "  make build                # Build all modules"
	@echo "  make lint                 # Run linter"
	@echo "  make clean                # Clean artifacts"
	@echo ""
	@echo "Tag Management:"
	@echo "  make tag v0.1.0           # Create and push tags"
	@echo "  make tag-create v0.1.0    # Create tags locally"
	@echo "  make tag-push v0.1.0      # Push tags to remote"
	@echo "  make tag-delete v0.1.0    # Delete tags"
	@echo "  make tag-minor            # Bump patch (0.1.0 -> 0.1.1)"
	@echo "  make tag-major            # Bump minor (0.1.0 -> 0.2.0)"
	@echo ""
	@echo "Development:"
	@echo "  make format               # Format code"
	@echo "  make mod-tidy             # Tidy all modules"

test: ## Run tests
	@echo Running tests...
	$(GO) test $(MODULE_PACKAGES)

coverage: ## Generate coverage
	@echo Running tests with coverage...
	$(GO) test $(MODULE_PACKAGES) -coverprofile=$(COVERPROFILE_ALL) -covermode=$(COVERMODE)

badge: coverage ## Generate coverage badge
	@echo Generating coverage badge...
	$(GO) run $(TOOLS_PKG)/badge@latest -in $(COVERPROFILE_ALL) -out $(BADGE_DIR)/coverage.svg -label $(BADGE_LABEL)
	@echo CI completed successfully!

lint: ## Run linter
	@echo "Running linter..."
	@$(GO) run $(LINT_TOOL) run --timeout=5m $(MODULE_PACKAGES)

build: ## Build all modules
	@echo Building all modules...
	cd core && $(GO) build -v ./... && cd ..
	cd validator && $(GO) build -v ./... && cd ..
	cd controller && $(GO) build -v ./... && cd ..
	cd pipes && $(GO) build -v ./... && cd ..
	cd guards && $(GO) build -v ./... && cd ..
	cd interceptors && $(GO) build -v ./... && cd ..
	cd exceptions && $(GO) build -v ./... && cd ..
	cd swagger && $(GO) build -v ./... && cd ..
	cd di && $(GO) build -v ./... && cd ..
	cd platform && $(GO) build -v ./... && cd ..
	@echo All modules built successfully!

clean: ## Clean artifacts
	@echo Cleaning...
	$(GO) clean -testcache
	$(GO) run $(TOOLS_PKG)/clean@latest $(COVERPROFILE_ALL) "*.coverage.out" "$(BADGE_DIR)/*.svg"
	@echo Cleaned!

format: ## Format code
	@echo Formatting code...
	$(GO) fmt ./...

mod-tidy: ## Tidy all modules
	@echo Tidying modules...
	cd core && $(GO) mod tidy && cd ..
	cd validator && $(GO) mod tidy && cd ..
	cd controller && $(GO) mod tidy && cd ..
	cd pipes && $(GO) mod tidy && cd ..
	cd guards && $(GO) mod tidy && cd ..
	cd interceptors && $(GO) mod tidy && cd ..
	cd exceptions && $(GO) mod tidy && cd ..
	cd swagger && $(GO) mod tidy && cd ..
	cd di && $(GO) mod tidy && cd ..
	cd platform && $(GO) mod tidy && cd ..
	@echo All modules tidied!

# Tag management
tag-create: ## Create tags locally
	$(GO) run $(TOOLS_PKG)/tag@latest --create "$(filter-out $@,$(MAKECMDGOALS))"

tag-push: ## Push tags to remote
	$(GO) run $(TOOLS_PKG)/tag@latest --push "$(filter-out $@,$(MAKECMDGOALS))"

tag-delete: ## Delete tags
	$(GO) run $(TOOLS_PKG)/tag@latest --delete "$(filter-out $@,$(MAKECMDGOALS))"

tag: ## Create and push tags
	$(GO) run $(TOOLS_PKG)/tag@latest --create --push "$(filter-out $@,$(MAKECMDGOALS))"

tag-minor: ## Bump patch version
	$(GO) run $(TOOLS_PKG)/tag@latest --bump patch

tag-major: ## Bump minor version
	$(GO) run $(TOOLS_PKG)/tag@latest --bump minor

tag-purge: ## Purge old tags
	$(GO) run $(TOOLS_PKG)/tag@latest --purge "$(filter-out $@,$(MAKECMDGOALS))"

# Prevent make from treating arguments as targets
%:
	@:
