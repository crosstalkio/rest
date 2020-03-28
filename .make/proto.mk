PROTOGENGO := $(GOPATH)/bin/protoc-gen-go

$(PROTOGENGO):
	go install google.golang.org/protobuf/cmd/protoc-gen-go

%.pb.go: %.proto $(PROTOGENGO)
	protoc --go_out=. $<

proto: clean/proto $(PBGO)

clean/proto:
	rm -f $(PBGO)

.PHONY: proto clean/proto
