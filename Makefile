
linter:
	docker run -t --rm -v $$(pwd):/app -w /app \
	-v $$(go env GOCACHE):/.cache/go-build -e GOCACHE=/.cache/go-build \
	-v $$(go env GOMODCACHE):/.cache/mod -e GOMODCACHE=/.cache/mod \
	-v ~/.cache/golangci-lint:/.cache/golangci-lint -e GOLANGCI_LINT_CACHE=/.cache/golangci-lint \
	golangci/golangci-lint:v2.6.2-alpine golangci-lint run --fix --config .golangci.yaml --timeout 5m --concurrency 4 

test:
	docker run -t --rm -v $$(pwd):/app -w /app \
	-v $$(go env GOCACHE):/.cache/go-build -e GOCACHE=/.cache/go-build \
	-v $$(go env GOMODCACHE):/.cache/mod -e GOMODCACHE=/.cache/mod \
	--entrypoint "" golang:1.25.0-alpine sh -c "go test -v -short -count=1 -p 4 -coverprofile=coverage.out ./... && go tool cover -func=coverage.out && go tool cover -html=coverage.out -o coverage.html"

integration-test:
	go test -v ./tests/integration/...

mail-smoke-test: build
	@echo "Starting GreenMail for smoke test..."
	docker compose -f docker-compose.mailtest.yml up -d
	@echo "Waiting for GreenMail to start..."
	sleep 5
	@echo "Running tests against local greenmail..."
	POSTERO_CONFIG_DIR="$$(pwd)/.tmp/mailtest-config" ./bin/pstr config validate
	POSTERO_CONFIG_DIR="$$(pwd)/.tmp/mailtest-config" ./bin/pstr compose -s "Makefile Smoke" --send
	POSTERO_CONFIG_DIR="$$(pwd)/.tmp/mailtest-config" ./bin/pstr sync --account local
	@echo "Smoke tests passed. Tearing down..."
	docker compose -f docker-compose.mailtest.yml down

build:
	go build -o bin/pstr ./cmd/pstr

run: build
	./bin/pstr

man: build
	./bin/pstr man ./docs/man

install: build man
	mkdir -p ~/.local/bin/
	cp bin/pstr ~/.local/bin/pstr
	mkdir -p ~/.local/share/man/man1
	cp docs/man/*.1 ~/.local/share/man/man1/ || true

