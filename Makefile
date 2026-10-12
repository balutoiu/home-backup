GO_IMAGE ?= golang:1.27.2-alpine
GO := go

ifneq ($(shell uname -s),Linux)
GO := docker run --rm \
	--user "$$(id -u):$$(id -g)" \
	-e HOME=/tmp \
	-e GOCACHE=/tmp/go-build \
	-e GOMODCACHE=/tmp/go-mod \
	-v "$(CURDIR):/workspace" \
	-w /workspace \
	$(GO_IMAGE) go
endif

.PHONY: build test test-lvm-vm lint fmt clean

build:
	mkdir -p build
	$(GO) build -trimpath -o ./build/home-backup ./cmd/home-backup

test:
	$(GO) test ./...

test-lvm-vm:
	./testdata/lvm-vm/run.sh

lint:
	$(GO) vet ./...

fmt:
	$(GO) fmt ./...

clean:
	rm -rf build
