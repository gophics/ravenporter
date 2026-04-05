.PHONY: build test test-race test-race-core test-cover test-integration test-integration-race test-all release-check lint bench bench-integration fuzz vet clean ci

ROOT_GOCACHE := $(CURDIR)/.gocache
TESTSUITE_GOCACHE := $(CURDIR)/testsuite/.gocache

build:
	set "GOCACHE=$(ROOT_GOCACHE)" && go build ./...

test:
	set "GOCACHE=$(ROOT_GOCACHE)" && go test ./... -count=1

test-race:
	set "GOCACHE=$(ROOT_GOCACHE)" && go test -race ./... -count=1

test-race-core:
	set "GOCACHE=$(ROOT_GOCACHE)" && go test -race ./ ./cache ./detect ./ir ./process ./internal/pipeline -count=1

test-cover:
	set "GOCACHE=$(ROOT_GOCACHE)" && go test -coverprofile=coverage.txt -covermode=atomic ./...
	set "GOCACHE=$(ROOT_GOCACHE)" && go tool cover -html=coverage.txt -o coverage.html

test-integration:
	cd testsuite && set "GOCACHE=$(TESTSUITE_GOCACHE)" && go test -tags integration ./... -count=1

test-integration-race:
	cd testsuite && set "GOCACHE=$(TESTSUITE_GOCACHE)" && go test -tags integration -race ./... -count=1

test-all: test test-integration

release-check: lint vet test test-integration test-race-core

vet:
	set "GOCACHE=$(ROOT_GOCACHE)" && go vet ./...

lint:
	golangci-lint run

bench:
	set "GOCACHE=$(ROOT_GOCACHE)" && go test -bench=. -benchmem -run=^$$ ./...

bench-integration:
	cd testsuite && set "GOCACHE=$(TESTSUITE_GOCACHE)" && go test -tags integration -bench=. -benchmem -run=^$$ ./...

fuzz:
	@echo "Run fuzz targets individually, e.g.:"
	@echo "  go test ./cache -fuzz=FuzzRead"
	@echo "  go test ./ir -fuzz=FuzzAssetGraphOps"
	@echo "  go test ./internal/decode/model/gltf -fuzz=FuzzDecode"

clean:
	go clean
	rm -f coverage.txt coverage.html

ci:
	set "GOCACHE=$(ROOT_GOCACHE)" && go vet ./...
	set "GOCACHE=$(ROOT_GOCACHE)" && go test ./... -count=1
	cd testsuite && set "GOCACHE=$(TESTSUITE_GOCACHE)" && go test -tags integration ./... -count=1
