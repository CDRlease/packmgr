.DEFAULT_GOAL := help

.PHONY: help build test install

VERSION ?= dev
OUTPUT ?= ./bin/packmgr

help:
	@echo "Targets:"
	@echo "  make build   VERSION=v0.1.0 OUTPUT=./bin/packmgr"
	@echo "  make test"
	@echo "  make install VERSION=v0.1.0"

build:
	mkdir -p "$(dir $(OUTPUT))"
	go build -ldflags="-X main.Version=$(VERSION)" -o "$(OUTPUT)" ./cmd/packmgr

test:
	go test ./...

install:
	VERSION="$(VERSION)" bash ./scripts/install.sh
