.PHONY: all help fmt vet lint test test-race vulncheck check

STATICCHECK := honnef.co/go/tools/cmd/staticcheck@v0.8.1
GOVULNCHECK := golang.org/x/vuln/cmd/govulncheck@v1.8.0

all: check

help: ## print this help
	@grep -E '^[a-zA-Z_:\\-]+:.*?## .*$$' Makefile | awk 'BEGIN {FS = ":.*?## "}; {printf "\033[36m%-20s\033[0m %s\n", $$1, $$2}'

fmt: ## fail if gofmt would change a file
	@files="$$(gofmt -l .)"; if [ -n "$$files" ]; then echo "$$files"; exit 1; fi

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

check: fmt vet lint test-race vulncheck ## everything CI runs, use make -k to see all failures
