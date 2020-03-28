PROTOS := $(wildcard */*.proto)
PBGO := $(PROTOS:.proto=.pb.go)

SAMPLE := sample/sample
GOFILES := go.mod $(wildcard *.go) $(wildcard */*.go)

all: $(PBGO) tidy $(SAMPLE)
	go build .

tidy:
	go mod tidy

$(SAMPLE): $(GOFILES)
	go build -o $@ ./sample

clean:
	rm -f sample/sample
	rm -f sample/*.pb.go

test: # -count=1 disables cache
	go test -v -race -count=1 .
	go test -v -race -count=1 ./sample

serve:
	go run ./sample

.PHONY: all tidy test clean serve

include .make/lint.mk
include .make/proto.mk
