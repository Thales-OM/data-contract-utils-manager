# Makefile for Go application

# Variables
BINARY_NAME = data-contract-utils-manager.exe
BUILD_DIR = ./build
DEPLOY_DIR = ../data-contract-utils

# Default target
all: build

# Build the Go application
build: force
	go build -o $(BUILD_DIR)/$(BINARY_NAME) main.go

# Force the build to always run
force:

# Deploy the application
deploy: clean
	cp $(BUILD_DIR)/$(BINARY_NAME) $(DEPLOY_DIR)

# Clean the deploy directory
clean:
	rm -rf $(DEPLOY_DIR)/* $(DEPLOY_DIR)/.*
