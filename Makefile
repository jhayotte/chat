
PROJECT_NAME := "chat"
PKG := "github.com/jhayotte/$(PROJECT_NAME)"
PKG_LIST := $(shell go list ${PKG}/... | grep -v /vendor/)
GO_FILES := $(shell find . -name '*.go' | grep -v /vendor/ | grep -v _test.go)
PROTO_FILES=$(shell find . -path '*.proto' | grep -v "vendor")
GOOGLE_APIS=github.com/grpc-ecosystem/grpc-gateway/third_party/googleapis
PROTOC_FLAGS=-I/usr/local/include -I. -I$(GOPATH)/src -I$(GOPATH)/src/$(GOOGLE_APIS) 

.PHONY: all dep build clean test coverage coverhtml lint protos

all: build

lint: ## Lint the files
	@golint -set_exit_status ${PKG_LIST}

protos: ## Build the proto file
	@echo "$@"
	@$(foreach file,$(PROTO_FILES),protoc $(PROTOC_FLAGS)  --go_out=plugins=grpc:. $(file);)
	@$(foreach file,$(PROTO_FILES),protoc $(PROTOC_FLAGS) --grpc-gateway_out=logtostderr=true,allow_delete_body=true:. $(file);)
	@$(foreach file,$(PROTO_FILES),protoc $(PROTOC_FLAGS) --swagger_out=logtostderr=true,allow_delete_body=true:. $(file);)

test: ## Run unittests
	@go test -short ${PKG_LIST}

race: dep ## Run data race detector
	@go test -race -short ${PKG_LIST}

dep: ## Get the dependencies
	@go get -v -d ./...

build: dep ## Build the binary file
	@go build -i -v $(PKG)

clean: ## Remove previous build
	@rm -f $(PROJECT_NAME)

help: ## Display this help screen
	@grep -h -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | awk 'BEGIN {FS = ":.*?## "}; {printf "\033[36m%-30s\033[0m %s\n", $$1, $$2}'