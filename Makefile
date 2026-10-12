.PHONY: all help fmt fix tidy vet lint test test-race vulncheck check

STATICCHECK := honnef.co/go/tools/cmd/staticcheck@v0.7.0-0.dev.0.20261009230814-452d5bb86b45
GOVULNCHECK := golang.org/x/vuln/cmd/govulncheck@v1.9.0

all: check

help: ## print this help
	@grep -E '^[a-zA-Z_:\\-]+:.*?## .*$$' Makefile | awk 'BEGIN {FS = ":.*?## "}; {printf "\033[36m%-20s\033[0m %s\n", $$1, $$2}'

fmt: ## fail if gofmt would change a file
	@files="$$(gofmt -l .)"; if [ -n "$$files" ]; then echo "$$files"; exit 1; fi

fix: ## fail if go fix would change a file
	go fix -diff ./...

tidy: ## fail if go mod tidy would change go.mod or go.sum
	go mod tidy -diff

vet: ## vet
	go vet ./...

lint: ## staticcheck
	go run $(STATICCHECK) ./...

test: ## test
	go test -v ./...

test-race: ## test with the race detector
	go test -race ./...

vulncheck: ## govulncheck
	go run $(GOVULNCHECK) ./...

check: fmt fix tidy vet lint test-race vulncheck ## everything CI runs, use make -k to see all failures
