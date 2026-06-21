.PHONY: build test lint install clean

build:
	go build -o bin/airraro-pp-cli ./cmd/airraro-pp-cli

test:
	go test ./...

lint:
	golangci-lint run

install:
	go install ./cmd/airraro-pp-cli

clean:
	rm -rf bin/

build-mcp:
	go build -o bin/airraro-pp-mcp ./cmd/airraro-pp-mcp

install-mcp:
	go install ./cmd/airraro-pp-mcp

build-all: build build-mcp
