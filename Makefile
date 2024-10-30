# 定义变量
BINARY_NAME = informer
VERSION ?= $(shell git describe --tags --always)
BUILD_DIR=build
CMD_DIR=cmd/informer

build-linux:
	@echo "Building for Linux..."
	GOOS=linux GOARCH=amd64 go build -o $(BUILD_DIR)/$(BINARY_NAME)-linux-amd64 $(CMD_DIR)/main.go
	@echo "Copying data directory..."
	cp -r data $(BUILD_DIR)/

build:
	@echo "Building for local system..."
	go build -o $(BUILD_DIR)/$(BINARY_NAME) $(CMD_DIR)/main.go
	@echo "Copying data directory..."
	cp -r data $(BUILD_DIR)/


clean:
	@echo "Cleaning up build files..."
	rm -rf $(BUILD_DIR)

run:
	make clean
	make build
	@echo "Running informer..."
	./$(BUILD_DIR)/$(BINARY_NAME)

default: build
