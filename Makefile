BIN     := tmuxizer
PKG     := ./cmd/tmuxizer
VERSION := $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
LDFLAGS := -s -w -X main.version=$(VERSION) -X main.source=$(CURDIR)

.PHONY: build install setup test lint fmt run clean

build:
	go build -ldflags '$(LDFLAGS)' -o bin/$(BIN) $(PKG)

install:
	go install -ldflags '$(LDFLAGS)' $(PKG)

# install.sh builds to ~/.local/bin and runs `tmuxizer setup`.
setup:
	./install.sh

test:
	go test -race ./...

lint:
	go vet ./...
	golangci-lint run

fmt:
	gofmt -w .

run:
	go run $(PKG)

clean:
	rm -rf bin dist
