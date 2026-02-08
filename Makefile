# Makefile for g
# Copyright 2025 Takuto Wada
# Copyright 2026 k-sub1995
# SPDX-License-Identifier: Apache-2.0

VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
LDFLAGS := -ldflags "-s -w -X main.version=$(VERSION) -X github.com/k-sub1995/g/cmd.version=$(VERSION)"
BINARY := g
BUILD_DIR := build

# Platforms for cross-compilation
PLATFORMS := \
	darwin/amd64 \
	darwin/arm64 \
	linux/amd64 \
	linux/arm64 \
	windows/amd64

.PHONY: all build clean test install cross-compile

# Default target
all: build

# Build for current platform
build:
	go build $(LDFLAGS) -o $(BINARY) .

# Run tests
test:
	go test -v ./...

# Install to GOPATH/bin
install:
	go install $(LDFLAGS) .

# Clean build artifacts
clean:
	rm -rf $(BINARY) $(BUILD_DIR)

# Cross-compile for all platforms
cross-compile: clean
	@mkdir -p $(BUILD_DIR)
	@for platform in $(PLATFORMS); do \
		GOOS=$${platform%/*} GOARCH=$${platform#*/} \
		go build $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY)-$${platform%/*}-$${platform#*/}$$([ "$${platform%/*}" = "windows" ] && echo ".exe") . ; \
		echo "Built: $(BUILD_DIR)/$(BINARY)-$${platform%/*}-$${platform#*/}$$([ "$${platform%/*}" = "windows" ] && echo ".exe")" ; \
	done

# Build for specific platform (e.g., make build-linux-amd64)
build-%:
	$(eval GOOS := $(word 1,$(subst -, ,$*)))
	$(eval GOARCH := $(word 2,$(subst -, ,$*)))
	@mkdir -p $(BUILD_DIR)
	GOOS=$(GOOS) GOARCH=$(GOARCH) go build $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY)-$(GOOS)-$(GOARCH)$(if $(filter windows,$(GOOS)),.exe) .
	@echo "Built: $(BUILD_DIR)/$(BINARY)-$(GOOS)-$(GOARCH)$(if $(filter windows,$(GOOS)),.exe)"

# Development: build and run
run:
	go run . $(ARGS)

# Check code quality
lint:
	go vet ./...
	@if command -v staticcheck >/dev/null 2>&1; then staticcheck ./...; fi

# Format code
fmt:
	go fmt ./...

# Show help
help:
	@echo "Available targets:"
	@echo "  build          - Build for current platform"
	@echo "  test           - Run tests"
	@echo "  install        - Install to GOPATH/bin"
	@echo "  clean          - Remove build artifacts"
	@echo "  cross-compile  - Build for all platforms"
	@echo "  build-GOOS-GOARCH - Build for specific platform (e.g., build-linux-amd64)"
	@echo "  run ARGS=...   - Build and run with arguments"
	@echo "  lint           - Run linters"
	@echo "  fmt            - Format code"
