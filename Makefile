VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
BINARY_NAME = discord-mcp
LDFLAGS = -s -w -X main.version=$(VERSION)
PREFIX ?= $(HOME)/.local

.PHONY: all build test lint vet vuln cover clean install

all: vet lint vuln test build

build:
	CGO_ENABLED=0 go build -ldflags="$(LDFLAGS)" -o $(BINARY_NAME) .

# Symlink (not copy) so the binary resolves to the repo and finds ./.env.
install: build
	mkdir -p $(PREFIX)/bin
	ln -sfn $(CURDIR)/$(BINARY_NAME) $(PREFIX)/bin/$(BINARY_NAME)

test:
	go test -race -coverprofile=coverage.out -covermode=atomic ./...
	@go tool cover -func=coverage.out | tail -1

cover: test
	go tool cover -html=coverage.out -o coverage.html

vet:
	go vet ./...

lint:
	golangci-lint run ./...

vuln:
	govulncheck ./...

clean:
	rm -f $(BINARY_NAME) coverage.out coverage.html
