.PHONY: compile
PROTOC_GEN_GO := $(GOPATH)/bin/protoc-gen-go

# If $GOPATH/bin/protoc-gen-go does not exist, we'll run this command to install it.
$(PROTOC_GEN_GO):
	@echo "Run protoc gen go"
	@go get -u github.com/golang/protobuf/protoc-gen-go

.PHONY: proto
proto:
	@echo ">> generating code from proto files"
	@./scripts/generate_proto.sh


APP_BIN = ./bin/pds-sample-client

${APP_BIN}: compile

recompile: compile

GOOS ?= "linux"
ARCH ?= "amd64"

compile:
	@echo "compiling pds-sample-client...."
	@env GOOS=$(GOOS) GOARCH=$(ARCH) go build -o ./bin/pds-sample-client ./

run: $(APP_BIN)
	@echo "Starting PDS client..."
	@./bin/pds-sample-client


.PHONY: clean
clean:
	@rm -rf ./tmp
	@rm -rf ./bin
	@rm -rf ./logs

