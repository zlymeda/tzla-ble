.DEFAULT_GOAL := all
SHELL := /bin/bash

all: prepush

prepush: tidy fmt vet

.PHONY: vet
vet:
	go vet ./...

.PHONY: fmt
fmt:
	go fmt ./...

.PHONY: tidy
tidy:
	go mod tidy

.PHONY: build
build:
	go build ./...

.PHONY: test
test:
	CGO_ENABLED=0 go build ./...
	CGO_ENABLED=1 go test -race -cover -v -mod=readonly ./... && echo -e "\033[32mSUCCESS\033[0m" || (echo -e "\033[31mFAILED\033[0m" && exit 1)

.PHONY: test_short
test_short:
	CGO_ENABLED=1 go test -race -cover -short -v -mod=readonly ./... && echo -e "\033[32mSUCCESS\033[0m" || (echo -e "\033[31mFAILED\033[0m" && exit 1)

.PHONY: clean_test
clean_test:
	go clean -testcache


.PHONY: go_list
go_list:
	go list -u -m all

.PHONY: go_update_all
go_update_all:
	go get -t -u ./...

.PHONY: download
download:
	@echo Download go.mod dependencies
	@go mod download

.PHONY: generate
generate:
	go generate -x ./...
