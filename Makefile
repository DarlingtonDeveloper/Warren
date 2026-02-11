.PHONY: build test lint ci install

build:
	go build -o bin/warren-server ./cmd/orchestrator
	go build -o bin/warren ./cmd/warren

install: build
	cp bin/warren /usr/local/bin/warren
	cp bin/warren-server /usr/local/bin/warren-server

test:
	go test ./... -v -race -count=1

lint:
	golangci-lint run ./...

ci: build test lint
