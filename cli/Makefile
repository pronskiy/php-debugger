.PHONY: build install clean test run

BINARY_NAME=dbg
INSTALL_PATH?=/usr/local/bin

build:
	go build -o bin/$(BINARY_NAME) .

install: build
	cp bin/$(BINARY_NAME) $(INSTALL_PATH)/$(BINARY_NAME)

clean:
	rm -rf bin/

test:
	go test ./...

run: build
	./bin/$(BINARY_NAME)

# Development
dev:
	go run .

# Cross-compile for different platforms
build-all:
	GOOS=linux GOARCH=amd64 go build -o bin/$(BINARY_NAME)-linux-amd64 .
	GOOS=linux GOARCH=arm64 go build -o bin/$(BINARY_NAME)-linux-arm64 .
	GOOS=darwin GOARCH=amd64 go build -o bin/$(BINARY_NAME)-darwin-amd64 .
	GOOS=darwin GOARCH=arm64 go build -o bin/$(BINARY_NAME)-darwin-arm64 .
	GOOS=windows GOARCH=amd64 go build -o bin/$(BINARY_NAME)-windows-amd64.exe .
