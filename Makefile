GO ?= go
PROTOC ?= protoc

GOGO_PROTOBUF_PATH := $(shell go list -e -f '{{.Dir}}' github.com/gogo/protobuf/gogoproto)/../
PROTO_FILES=$(shell find . -type f -not -path "./vendor/*" -name "*.proto")
PWD=$(shell pwd)
DATE=$(shell date +%y%m%d)
REMOTE_CONTAINER_NAME=ghcr.io/ibalajiarun/go-consensus

.PHONY: all
all: build

.PHONY: test
test: sgx
	LD_LIBRARY_PATH=$$LD_LIBRARY_PATH:/opt/sgxsdk/lib64:$(PWD)/utils/sgx/enclave_trinc:$(PWD)/utils/sgx/enclave_trudep:$(PWD)/utils/sgx/enclave_threshsign $(GO) test -mod=vendor -v ./protocols/sbft/...

.PHONY: proto
proto:
	@echo "Generating proto files..."
	@for target in $(PROTO_FILES) ; do \
		echo $$target ; \
		$(PROTOC) --gogofaster_out=paths=source_relative,plugins=grpc:. -I. -I$(GOGO_PROTOBUF_PATH) $$target ; \
	done
	@echo "Done generating proto files."

TARGETS := peer client master erunner
TARGETDIR := bin

.PHONY: build
build: clean sgx
	@echo "Go: Building..."
	@for target in $(TARGETS) ; do \
		$(GO) build -mod=vendor -o "$(TARGETDIR)/$$target" ./cmd/$$target || exit 1; \
	done
	@echo "Go: Build complete."

.PHONY: sgx
sgx:
	@echo "SGX: Building SGX..."
	@$(MAKE) -C enclaves/ TARGET=lib
	@echo "SGX: Build complete."

.PHONY: clean
clean:
	@$(RM) *.pb.*
	@$(RM) -r $(TARGETDIR)
	@$(MAKE) -C enclaves/ TARGET=clean

.PHONY: erunner
erunner: cmd/erunner/erunner.go
	$(GO) build -mod=vendor -o "$(TARGETDIR)/$@" $?

container:
	@docker build -t $(REMOTE_CONTAINER_NAME) -f build/dockerfiles/Dockerfile .
	@docker tag $(REMOTE_CONTAINER_NAME) $(REMOTE_CONTAINER_NAME):$(DATE)
	@docker push $(REMOTE_CONTAINER_NAME):$(DATE)
	@docker push $(REMOTE_CONTAINER_NAME):latest