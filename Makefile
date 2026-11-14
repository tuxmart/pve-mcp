.PHONY: build test lint fmt check smoke

build:
	go build ./...

test:
	go test -race ./...

lint:
	golangci-lint run

fmt:
	go fmt ./...

check: lint test

# Starts the server and checks initialize + tools/list over stdio.
smoke:
	go test -tags smoke -run TestSmoke ./cmd/pve-mcp/
