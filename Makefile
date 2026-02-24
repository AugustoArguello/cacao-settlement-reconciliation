.PHONY: all run test test-cover fmt lint clean build

all: fmt lint test

run:
	go run cmd/server/main.go

build:
	go build -o bin/server cmd/server/main.go

test:
	go test ./... -covermode=atomic -coverpkg=./... -count=1 -race -v

test-cover:
	go test ./... -covermode=atomic -coverprofile=coverage.out -coverpkg=./... -count=1
	go tool cover -html=coverage.out -o coverage.html

fmt:
	go fmt ./...

lint:
	golangci-lint run ./...

clean:
	rm -rf bin/ coverage.out coverage.html
