.PHONY: help build test test-race vet cover cover-html mutation clean

# Pilot mutation testing surface — keep narrow, expand only when the score is stable.
MUTATION_PKGS := ./pkg/dag/... ./internal/classifier/... ./internal/checker/...

help:
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | sort | awk 'BEGIN{FS=":.*?## "}{printf "\033[36m%-15s\033[0m %s\n", $$1, $$2}'

build: ## Compile the CLI to ./reify
	go build -o reify ./cmd/reify

vet: ## Run go vet
	go vet ./...

test: ## Run all tests
	go test ./...

test-race: ## Run all tests with race detector
	go test -race ./...

cover: ## Run tests with coverage and print summary
	go test -race -coverprofile=coverage.out -covermode=atomic ./...
	@go tool cover -func=coverage.out | tail -1

cover-html: cover ## Open the HTML coverage report
	go tool cover -html=coverage.out

mutation: ## Run mutation testing on the pilot packages (requires gremlins)
	@command -v gremlins >/dev/null 2>&1 || { \
		echo "gremlins not found. Install with: go install github.com/go-gremlins/gremlins/cmd/gremlins@latest"; \
		exit 1; \
	}
	gremlins unleash $(MUTATION_PKGS)

clean: ## Remove build artifacts
	rm -f reify coverage.out
