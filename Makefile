# Makefile for thermostat-telemetry-reader
# This makefile provides targets that mirror the CI pipeline and help with development

.PHONY: help test lint security vulnerability-check build clean setup deps verify mod-tidy-check all ci-local clean-template

# =============================================================================
# Configuration
# =============================================================================

GO_VERSION := 1.24.4
BINARY_NAME := thermostat-telemetry-reader
BUILD_DIR := ./bin

# =============================================================================
# Help
# =============================================================================

## help: Display this help message
help:
	@echo "Available targets:"
	@echo ""
	@echo "Development targets:"
	@echo "  setup              - Install required tools and dependencies via asdf"
	@echo "  deps               - Download and verify Go dependencies"
	@echo "  clean              - Remove build artifacts"
	@echo "  clean-template     - Clean up template code to prepare for new project"
	@echo ""
	@echo "Tool management targets:"
	@echo "  update-tool-versions - Update .tool-versions with latest versions"
	@echo "  pin-tool-version   - Pin a specific tool version"
	@echo "  unpin-tool-version - Unpin a specific tool version"
	@echo "  verify-tools       - Verify all development tools are working"
	@echo ""
	@echo "Testing targets (mirror CI):"
	@echo "  test               - Run all tests with race detection and coverage"
	@echo "  lint               - Run golangci-lint"
	@echo "  security           - Run Gosec security scanner"
	@echo "  vulnerability-check- Run govulncheck for vulnerability scanning"
	@echo "  build              - Build binaries for multiple platforms"
	@echo "  mod-tidy-check     - Check if go mod tidy is needed"
	@echo ""
	@echo "Docker targets:"
	@echo "  docker-build       - Build Docker image"
	@echo "  docker-run         - Run Docker container"
	@echo "  docker-compose-up  - Start services with docker-compose"
	@echo "  docker-compose-down- Stop services with docker-compose"
	@echo ""
	@echo "Code generation targets:"
	@echo "  generate           - Generate code (if using go generate)"
	@echo "  benchmark          - Run benchmarks"
	@echo "  profile            - Run tests with profiling"
	@echo ""
	@echo "Release management targets:"
	@echo "  release-patch-rc   - Create a patch release candidate (any branch, clean & synced)"
	@echo "  release-patch      - Create a patch release (main branch only, clean & synced)"
	@echo "  release-minor-rc   - Create a minor release candidate (any branch, clean & synced)"
	@echo "  release-minor      - Create a minor release (main branch only, clean & synced)"
	@echo "  release-major-rc   - Create a major release candidate (any branch, clean & synced)"
	@echo "  release-major      - Create a major release (main branch only, clean & synced)"
	@echo "  list-versions      - List all version tags"
	@echo "  list-rc-versions   - List all release candidate tags"
	@echo "  next-version       - Show next version (usage: make next-version TYPE=patch)"
	@echo "  next-rc-version    - Show next RC version (usage: make next-rc-version TYPE=patch)"
	@echo ""
	@echo "Convenience targets:"
	@echo "  all                - Run all quality checks (test, lint, security, vuln-check)"
	@echo "  ci-local           - Run the same checks as CI pipeline"

# =============================================================================
# Development Setup
# =============================================================================

## setup: Install required development tools via asdf
setup: check-go-version
	@echo "Installing development tools via asdf..."
	@asdf plugin add golangci-lint || true
	@asdf plugin add gosec || true
	@echo "Installing asdf tools..."
	@asdf install golang || echo "Go already installed"
	@asdf install golangci-lint || echo "golangci-lint already installed"
	@asdf install gosec || echo "gosec already installed"
	@asdf reshim
	@echo "Installing Go tools..."
	@go install golang.org/x/vuln/cmd/govulncheck@latest
	@echo "Development tools installed successfully!"
	@make verify-tools

## check-go-version: Verify Go version matches project requirements
check-go-version:
	@echo "Checking Go version..."
	@if ! go version | grep -qE "go1\.(2[4-9]|[3-9][0-9])"; then \
		echo "Error: Go version 1.24+ required. Current version:"; \
		go version; \
		echo "Please update Go using: asdf install"; \
		exit 1; \
	fi
	@echo "Go version check passed!"

## deps: Download and verify dependencies
deps:
	@echo "Downloading dependencies..."
	go mod download
	@echo "Verifying dependencies..."
	go mod verify
	@echo "Dependencies ready!"

## verify: Verify the module and dependencies
verify:
	@echo "Verifying module..."
	go mod verify
	@echo "Module verification completed!"

# =============================================================================
# Tool Management
# =============================================================================

## verify-tools: Verify all development tools are working correctly
verify-tools:
	@echo "Verifying development tools..."
	@echo "Go version: $$(go version)"
	@echo "golangci-lint version: $$(golangci-lint version)"
	@echo "govulncheck version: $$(govulncheck -version 2>/dev/null || echo 'govulncheck not available')"
	@echo "gosec version: $$(gosec -version 2>/dev/null || echo 'gosec not available')"
	@echo "Tool verification completed!"

## update-tool-versions: Update .tool-versions with latest versions (respects pinned versions)
update-tool-versions:
	@echo "Updating .tool-versions with latest versions..."
	@if [ ! -f .tool-versions ]; then \
		echo "Error: Error: .tool-versions file not found"; \
		exit 1; \
	fi
	@cp .tool-versions .tool-versions.backup
	@while IFS= read -r line; do \
		if echo "$$line" | grep -q "#pinned"; then \
			echo "$$line" >> .tool-versions.tmp; \
			echo "Keeping pinned: $$line"; \
		else \
			tool=$$(echo "$$line" | awk '{print $$1}'); \
			if [ -n "$$tool" ] && [ "$$tool" != "#" ]; then \
				latest=$$(asdf latest "$$tool" 2>/dev/null || echo "unknown"); \
				if [ "$$latest" != "unknown" ] && ! echo "$$latest" | grep -q "unable to load\|does not have\|unknown"; then \
					echo "$$tool $$latest" >> .tool-versions.tmp; \
					echo "Updated $$tool to $$latest"; \
				else \
					echo "$$line" >> .tool-versions.tmp; \
					echo "Keeping $$line (no update available)"; \
				fi; \
			else \
				echo "$$line" >> .tool-versions.tmp; \
			fi; \
		fi; \
	done < .tool-versions
	@mv .tool-versions.tmp .tool-versions
	@echo "Updated .tool-versions successfully!"
	@echo "Run 'asdf install' to install updated versions"

## pin-tool-version: Pin a specific tool version (usage: make pin-tool-version TOOL=golangci-lint VERSION=2.3.0)
pin-tool-version:
	@if [ -z "$(TOOL)" ] || [ -z "$(VERSION)" ]; then \
		echo "Error: Error: Usage: make pin-tool-version TOOL=toolname VERSION=version"; \
		echo "Example: make pin-tool-version TOOL=golangci-lint VERSION=2.3.0"; \
		exit 1; \
	fi
	@echo "Pinning $(TOOL) to version $(VERSION)..."
	@if [ ! -f .tool-versions ]; then \
		echo "Error: Error: .tool-versions file not found"; \
		exit 1; \
	fi
	@sed -i.bak "s/^$(TOOL) .*/$(TOOL) $(VERSION) #pinned/" .tool-versions
	@rm -f .tool-versions.bak
	@echo "Pinned $(TOOL) to $(VERSION)"

## unpin-tool-version: Unpin a specific tool version (usage: make unpin-tool-version TOOL=golangci-lint)
unpin-tool-version:
	@if [ -z "$(TOOL)" ]; then \
		echo "Error: Error: Usage: make unpin-tool-version TOOL=toolname"; \
		echo "Example: make unpin-tool-version TOOL=golangci-lint"; \
		exit 1; \
	fi
	@echo "Unpinning $(TOOL)..."
	@if [ ! -f .tool-versions ]; then \
		echo "Error: Error: .tool-versions file not found"; \
		exit 1; \
	fi
	@sed -i.bak "s/^$(TOOL) .* #pinned/$(TOOL) $$(asdf latest $(TOOL) 2>/dev/null || echo 'unknown')/" .tool-versions
	@rm -f .tool-versions.bak
	@echo "Unpinned $(TOOL)"

# =============================================================================
# Testing and Quality Checks
# =============================================================================

## test: Run tests with race detection and coverage
test:
	@echo "Running tests..."
	go test -v -race -coverprofile=coverage.out ./...
	@echo "Tests completed!"
	@echo "Coverage report:"
	go tool cover -func=coverage.out

## lint: Run golangci-lint
lint: check-golangci-lint-version
	@echo "Running linter..."
	golangci-lint run --timeout=10m
	@echo "Linting completed!"

## check-golangci-lint-version: Verify golangci-lint version is correct
check-golangci-lint-version:
	@echo "Checking golangci-lint version..."
	@if ! golangci-lint version | grep -q "version 2"; then \
		echo "Error: Error: golangci-lint version 2.x required. Current version:"; \
		golangci-lint version; \
		echo "Please run: asdf reshim golangci-lint"; \
		exit 1; \
	fi
	@echo "golangci-lint version check passed!"

## security: Run Gosec security scanner
security:
	@echo "Running security scan..."
	gosec -no-fail -fmt text ./...
	@echo "Security scan completed!"

## vulnerability-check: Run govulncheck
vulnerability-check:
	@echo "Checking for vulnerabilities..."
	govulncheck ./...
	@echo "Vulnerability check completed!"

## mod-tidy-check: Check if go mod tidy is needed
mod-tidy-check:
	@echo "Checking if go mod tidy is needed..."
	@go mod tidy
	@git diff --exit-code go.mod go.sum || { \
		echo "Error: Error: go.mod or go.sum is not tidy. Please run 'go mod tidy' and commit the changes."; \
		exit 1; \
	}
	@echo "go.mod and go.sum are tidy!"

# =============================================================================
# Build and Release
# =============================================================================

## build: Build binaries for multiple platforms
build:
	@echo "Building binaries..."
	@mkdir -p $(BUILD_DIR)
	@echo "Building for Linux AMD64..."
	@GOOS=linux GOARCH=amd64 go build -o $(BUILD_DIR)/$(BINARY_NAME)-linux-amd64 ./cmd/ttr
	@echo "Building for Linux ARM64..."
	@GOOS=linux GOARCH=arm64 go build -o $(BUILD_DIR)/$(BINARY_NAME)-linux-arm64 ./cmd/ttr
	@echo "Building for macOS AMD64..."
	@GOOS=darwin GOARCH=amd64 go build -o $(BUILD_DIR)/$(BINARY_NAME)-darwin-amd64 ./cmd/ttr
	@echo "Building for macOS ARM64..."
	@GOOS=darwin GOARCH=arm64 go build -o $(BUILD_DIR)/$(BINARY_NAME)-darwin-arm64 ./cmd/ttr
	@echo "Building for Windows AMD64..."
	@GOOS=windows GOARCH=amd64 go build -o $(BUILD_DIR)/$(BINARY_NAME)-windows-amd64.exe ./cmd/ttr
	@echo "All builds completed!"
	@echo "Built binaries:"
	@ls -la $(BUILD_DIR)/

# =============================================================================
# Docker
# =============================================================================

## docker-build: Build Docker image
docker-build:
	@echo "Building Docker image..."
	docker build -t $(BINARY_NAME):latest .
	@echo "Docker image built successfully!"

## docker-run: Run Docker container
docker-run:
	@echo "Running Docker container..."
	docker run -p 8080:8080 $(BINARY_NAME):latest

## docker-compose-up: Start services with docker-compose
docker-compose-up:
	@echo "Starting services with docker-compose..."
	docker-compose up -d
	@echo "Services started!"

## docker-compose-down: Stop services with docker-compose
docker-compose-down:
	@echo "Stopping services with docker-compose..."
	docker-compose down
	@echo "Services stopped!"

# =============================================================================
# Code Generation and Analysis
# =============================================================================

## generate: Generate code (if using go generate)
generate:
	@echo "Generating code..."
	go generate ./...
	@echo "Code generation completed!"

## benchmark: Run benchmarks
benchmark:
	@echo "Running benchmarks..."
	go test -bench=. -benchmem ./...
	@echo "Benchmarks completed!"

## profile: Run tests with profiling
profile:
	@echo "Running tests with profiling..."
	go test -cpuprofile=cpu.prof -memprofile=mem.prof ./...
	@echo "Profiling completed!"

# =============================================================================
# Cleanup
# =============================================================================

## clean: Remove build artifacts and coverage files
clean:
	@echo "Cleaning build artifacts..."
	@rm -rf $(BUILD_DIR)
	@rm -f coverage.out
	@rm -f results.sarif
	@echo "Clean completed!"

# =============================================================================
# Release Management
# =============================================================================

## release-patch-rc: Create a patch release candidate
release-patch-rc:
	@echo "Creating patch release candidate..."
	@$(MAKE) _validate-git-status
	@$(MAKE) _validate-branch-sync
	@$(MAKE) _create-release-candidate TYPE=patch

## release-patch: Create a patch release
release-patch:
	@echo "Creating patch release..."
	@$(MAKE) _validate-git-status
	@$(MAKE) _validate-branch-sync
	@$(MAKE) _validate-release-branch
	@$(MAKE) _create-release TYPE=patch

## release-minor-rc: Create a minor release candidate
release-minor-rc:
	@echo "Creating minor release candidate..."
	@$(MAKE) _validate-git-status
	@$(MAKE) _validate-branch-sync
	@$(MAKE) _create-release-candidate TYPE=minor

## release-minor: Create a minor release
release-minor:
	@echo "Creating minor release..."
	@$(MAKE) _validate-git-status
	@$(MAKE) _validate-branch-sync
	@$(MAKE) _validate-release-branch
	@$(MAKE) _create-release TYPE=minor

## release-major-rc: Create a major release candidate
release-major-rc:
	@echo "Creating major release candidate..."
	@$(MAKE) _validate-git-status
	@$(MAKE) _validate-branch-sync
	@$(MAKE) _create-release-candidate TYPE=major

## release-major: Create a major release
release-major:
	@echo "Creating major release..."
	@$(MAKE) _validate-git-status
	@$(MAKE) _validate-branch-sync
	@$(MAKE) _validate-release-branch
	@$(MAKE) _create-release TYPE=major

## _validate-release-branch: Internal target to validate we're on main branch
_validate-release-branch:
	@current_branch=$$(git branch --show-current); \
	if [ "$$current_branch" != "main" ] && [ "$$current_branch" != "master" ]; then \
		echo "Error: Must be on main or master branch to create releases. Current branch: $$current_branch"; \
		echo "Please switch to main branch: git checkout main"; \
		exit 1; \
	fi; \
	echo "Release branch validation passed!"

## _validate-git-status: Internal target to validate git working directory is clean
_validate-git-status:
	@echo "Checking git working directory status..."; \
	if ! git diff --quiet; then \
		echo "Error: Working directory has uncommitted changes"; \
		echo "Please commit or stash your changes before creating a release"; \
		git status --short; \
		exit 1; \
	fi; \
	if ! git diff --cached --quiet; then \
		echo "Error: Staging area has uncommitted changes"; \
		echo "Please commit or unstage your changes before creating a release"; \
		git status --short; \
		exit 1; \
	fi; \
	echo "Git working directory is clean!"

## _validate-branch-sync: Internal target to validate branch is up to date with origin
_validate-branch-sync:
	@echo "Checking if branch is up to date with origin..."; \
	git fetch origin; \
	current_branch=$$(git branch --show-current); \
	upstream=$$(git rev-parse --abbrev-ref --symbolic-full-name @{u} 2>/dev/null || echo "origin/$$current_branch"); \
	if [ -z "$$upstream" ]; then \
		echo "Error: No upstream branch found for $$current_branch"; \
		echo "Please set upstream: git push --set-upstream origin $$current_branch"; \
		exit 1; \
	fi; \
	local_commit=$$(git rev-parse HEAD); \
	remote_commit=$$(git rev-parse $$upstream); \
	if [ "$$local_commit" != "$$remote_commit" ]; then \
		echo "Error: Branch $$current_branch is not up to date with $$upstream"; \
		echo "Please pull the latest changes: git pull origin $$current_branch"; \
		echo "Or push your local changes: git push origin $$current_branch"; \
		exit 1; \
	fi; \
	echo "Branch is up to date with origin!"

## _get-latest-version: Internal target to get the latest version tag (excluding RCs)
_get-latest-version:
	@latest_tag=$$(git tag --list | grep -E '^v[0-9]+\.[0-9]+\.[0-9]+$$' | sort -V | tail -1); \
	if [ -z "$$latest_tag" ]; then \
		echo "v0.0.0"; \
	else \
		echo "$$latest_tag"; \
	fi

## _get-next-version: Internal target to calculate next version (usage: make _get-next-version TYPE=patch)
_get-next-version:
	@latest=$$($(MAKE) _get-latest-version | sed 's/v//'); \
	if [ -z "$$latest" ] || [ "$$latest" = "v0.0.0" ]; then \
		case "$(TYPE)" in \
			patch) echo "v0.0.1" ;; \
			minor) echo "v0.1.0" ;; \
			major) echo "v1.0.0" ;; \
		esac; \
	else \
		major=$$(echo $$latest | cut -d. -f1); \
		minor=$$(echo $$latest | cut -d. -f2); \
		patch=$$(echo $$latest | cut -d. -f3); \
		case "$(TYPE)" in \
			patch) echo "v$$major.$$minor.$$((patch + 1))" ;; \
			minor) echo "v$$major.$$((minor + 1)).0" ;; \
			major) echo "v$$((major + 1)).0.0" ;; \
		esac; \
	fi

## _get-next-rc-version: Internal target to calculate next RC version (usage: make _get-next-rc-version TYPE=patch)
_get-next-rc-version:
	@base_version=$$($(MAKE) _get-next-version TYPE=$(TYPE)); \
	rc_pattern="$$base_version-rc"; \
	rc_count=$$(git tag --list | grep "^$$rc_pattern" | wc -l | tr -d ' '); \
	if [ "$$rc_count" -eq 0 ]; then \
		echo "$$base_version-rc1"; \
	else \
		echo "$$base_version-rc$$((rc_count + 1))"; \
	fi

## _create-release-candidate: Internal target to create and push RC tag (usage: make _create-release-candidate TYPE=patch)
_create-release-candidate:
	@rc_version=$$($(MAKE) _get-next-rc-version TYPE=$(TYPE)); \
	echo "Creating release candidate tag: $$rc_version"; \
	git tag $$rc_version; \
	echo "Pushing tag to origin..."; \
	git push origin $$rc_version; \
	echo "Release candidate $$rc_version created and pushed!"

## _create-release: Internal target to create and push release tag (usage: make _create-release TYPE=patch)
_create-release:
	@release_version=$$($(MAKE) _get-next-version TYPE=$(TYPE)); \
	echo "Creating release tag: $$release_version"; \
	git tag $$release_version; \
	echo "Pushing tag to origin..."; \
	git push origin $$release_version; \
	echo "Release $$release_version created and pushed!"

## list-versions: List all version tags
list-versions:
	@echo "All version tags:"
	@git tag --list | grep -E '^v[0-9]+\.[0-9]+\.[0-9]+' | sort -V

## list-rc-versions: List all release candidate tags
list-rc-versions:
	@echo "All release candidate tags:"
	@git tag --list | grep -E '^v[0-9]+\.[0-9]+\.[0-9]+-rc[0-9]+' | sort -V

## next-version: Show what the next version would be (usage: make next-version TYPE=patch)
next-version:
	@next=$$($(MAKE) _get-next-version TYPE=$(TYPE)); \
	echo "Next $(TYPE) version would be: $$next"

## next-rc-version: Show what the next RC version would be (usage: make next-rc-version TYPE=patch)
next-rc-version:
	@next_rc=$$($(MAKE) _get-next-rc-version TYPE=$(TYPE)); \
	echo "Next $(TYPE) RC version would be: $$next_rc"

# =============================================================================
# Convenience Targets
# =============================================================================

## all: Run all quality checks
all: deps test lint security vulnerability-check mod-tidy-check
	@echo "All quality checks passed!"

## ci-local: Run the same checks as CI pipeline
ci-local: all build
	@echo "Local CI pipeline completed successfully!"

# Default target
.DEFAULT_GOAL := help
