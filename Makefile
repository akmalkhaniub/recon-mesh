.PHONY: all build run test bench clean

GO ?= go

all: build test

build:
	@echo "Building ReconMesh server..."
	$(GO) build -v -o bin/server.exe ./cmd/server

run: build
	./bin/server.exe

test:
	@echo "Running reconciliation test suite..."
	$(GO) test -v ./tests/...

bench:
	@echo "Running high-volume 10,000 transaction benchmark..."
	$(GO) test -v -bench=. -benchmem ./tests/...

clean:
	@rm -rf bin/
