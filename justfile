# use with https://github.com/casey/just
set shell := ["bash", "-cu"]

# run all tests (no cache)
test:
    go test -count=1 ./...

# run tests with verbose output (no cache)
test-v:
    go test -v -count=1 ./...

# run unit tests only (skip integration tests, no cache)
test-unit:
    go test -short -count=1 ./...

# run integration tests (requires dynamodb-local running, no cache)
test-integration:
    go test -v -count=1 -run Integration ./...

# run property-based tests (no cache)
test-property:
    go test -v -count=1 -run Property ./...

# start dynamodb-local for testing (idempotent)
db-up:
    @if docker ps -q -f name=dynamodb-local-test | grep -q .; then \
        echo "dynamodb-local already running"; \
    elif lsof -i :8000 -sTCP:LISTEN >/dev/null 2>&1; then \
        echo "port 8000 already in use, assuming dynamodb-local is available"; \
    else \
        docker compose -f docker-compose.test.yml up -d; \
    fi

# stop dynamodb-local (idempotent)
db-down:
    @if docker ps -aq -f name=dynamodb-local-test | grep -q .; then \
        docker compose -f docker-compose.test.yml down; \
    else \
        echo "dynamodb-local not running"; \
    fi

# seed test data into dynamodb-local
db-seed:
    bash test/seed_data.sh

# clean test data from dynamodb-local
db-clean:
    bash test/seed_data.sh clean

# full local test environment: start db, seed, run tests
test-full: db-up db-seed test

# build the binary
build:
    go build -o dist/querydb .

# run goreleaser in snapshot mode (no publish)
release-snapshot:
    goreleaser release --snapshot --clean

# run goreleaser for real
release:
    goreleaser release --clean

# run go vet
vet:
    go vet ./...

# tidy go modules
tidy:
    go mod tidy

# install querydb to /usr/local/bin
install:
    bash install.sh

# clean build artifacts
clean:
    rm -rf dist/
