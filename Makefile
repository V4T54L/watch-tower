.PHONY: all build test lint clean docker-build docker-push compose-up compose-down compose-logs help

# Variables
APP_NAME := watch-tower
INGEST_IMG_NAME := ${APP_NAME}-ingest
CONSUMER_IMG_NAME := ${APP_NAME}-consumer
TAG ?= latest
REGISTRY ?= your-registry # Replace with your container registry

all: build

# ====================================================================================
# Go Commands
# ====================================================================================

## build: Build the Go binaries
build:
	@echo "--> Building Go binaries..."
	@go build -o bin/ingest ./cmd/ingest
	@go build -o bin/consumer ./cmd/consumer

## test: Run unit tests with coverage
test:
	@echo "--> Running tests..."
	@go test -v -race -cover ./...

## lint: Run the linter
lint:
	@echo "--> Linting code..."
	@if ! command -v golangci-lint &> /dev/null; then \
		echo "golangci-lint not found. Please install it: https://golangci-lint.run/usage/install/"; \
		exit 1; \
	fi
	@golangci-lint run

## clean: Clean up build artifacts
clean:
	@echo "--> Cleaning up..."
	@rm -rf ./bin ./coverage.out

# ====================================================================================
# Docker Commands
# ====================================================================================

## docker-build: Build Docker images for ingest and consumer services
docker-build:
	@echo "--> Building Docker images..."
	@docker build -f Dockerfile.ingest -t ${INGEST_IMG_NAME}:${TAG} .
	@docker build -f Dockerfile.consumer -t ${CONSUMER_IMG_NAME}:${TAG} .

## docker-push: Push Docker images to the registry
docker-push:
	@echo "--> Pushing Docker images to ${REGISTRY}..."
	@docker tag ${INGEST_IMG_NAME}:${TAG} ${REGISTRY}/${INGEST_IMG_NAME}:${TAG}
	@docker tag ${CONSUMER_IMG_NAME}:${TAG} ${REGISTRY}/${CONSUMER_IMG_NAME}:${TAG}
	@docker push ${REGISTRY}/${INGEST_IMG_NAME}:${TAG}
	@docker push ${REGISTRY}/${CONSUMER_IMG_NAME}:${TAG}

# ====================================================================================
# Docker Compose Commands
# ====================================================================================

## compose-up: Start all services using Docker Compose
compose-up:
	@echo "--> Starting services with Docker Compose..."
	@docker-compose up -d --build

## compose-down: Stop and remove all services from Docker Compose
compose-down:
	@echo "--> Stopping services with Docker Compose..."
	@docker-compose down

## compose-logs: Follow logs from all services
compose-logs:
	@echo "--> Following logs..."
	@docker-compose logs -f

# ====================================================================================
# Help
# ====================================================================================

## help: Show this help message
help:
	@echo "Usage: make <target>"
	@echo ""
	@echo "Targets:"
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | sort | awk 'BEGIN {FS = ":.*?## "}; {printf "\033[36m%-20s\033[0m %s\n", $$1, $$2}'

.DEFAULT_GOAL := help

