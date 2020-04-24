PROTOS := $(wildcard */*.proto)
PBGO := $(PROTOS:.proto=.pb.go)

SAMPLE := sample/sample
GOFILES := go.mod $(wildcard *.go) $(wildcard */*.go)

all: $(PBGO) $(SAMPLE)
	go build .

include .make/golangci-lint.mk
include .make/protoc.mk
include .make/protoc-gen-go.mk

tidy:
	go mod tidy

lint: $(GOLANGCI_LINT)
	$(realpath $(GOLANGCI_LINT)) run

$(SAMPLE): $(GOFILES)
	go build -o $@ ./sample

clean/proto:
	rm -f $(PBGO)

clean: clean/golangci-lint clean/protoc clean/protoc-gen-go clean/proto
	rm -f go.sum
	rm -f $(SAMPLE)

test: # -count=1 disables cache
	go test -v -race -count=1 .
	go test -v -race -count=1 ./sample

serve:
	go run ./sample

.PHONY: all tidy lint clean test serve
