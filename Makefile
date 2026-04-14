.DEFAULT_GOAL := help

.PHONY: help build test

VERSION ?= dev
OUTPUT ?= ./bin/packmgr

help:
	@echo "Targets:"
	@echo "  make build   VERSION=v0.1.0 OUTPUT=./bin/packmgr"
	@echo "  make test"

build:
	mkdir -p "$(dir $(OUTPUT))"
	go build -ldflags="-X main.Version=$(VERSION)" -o "$(OUTPUT)" ./cmd/packmgr

test:
	go test ./...
