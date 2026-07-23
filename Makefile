PROTO_DIR := proto
GO_GEN_DIR := protogen
PROTO_FILE := $(PROTO_DIR)/pds.proto
APP_BIN := bin/pds-sample-client

.PHONY: generate install-tools clean regenerate build run

generate:
	mkdir -p $(GO_GEN_DIR)
	protoc \
		--proto_path=$(PROTO_DIR) \
		--go_out=$(GO_GEN_DIR) \
		--go_opt=paths=source_relative \
		--go-grpc_out=$(GO_GEN_DIR) \
		--go-grpc_opt=paths=source_relative \
		$(PROTO_FILE)

install-tools:
	go install google.golang.org/protobuf/cmd/protoc-gen-go@latest
	go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@latest

clean:
	rm -rf $(GO_GEN_DIR) $(APP_BIN)

regenerate: clean generate

build:
	go build -o $(APP_BIN) .

run: build
	./$(APP_BIN)