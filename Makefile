PROTOS := $(wildcard */*.proto)
PBGO := $(PROTOS:.proto=.pb.go)

PROTOGENGO := $(GOPATH)/bin/protoc-gen-go

all: $(PBGO)
	go mod tidy
	go build .
	go build -o sample/sample ./sample

$(PROTOGENGO):
	go install google.golang.org/protobuf/cmd/protoc-gen-go

%.pb.go: %.proto
	protoc --go_out=. $<

run:
	go run ./sample

test:
	go test -count=1 .
	go test -count=1 ./sample

clean:
	rm -f sample/sample
	rm -f sample/*.pb.go

.PHONY: all run test clean
