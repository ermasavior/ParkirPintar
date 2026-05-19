PROTO_DIR   := proto
GEN_DIR     := gen
GOPATH      := $(shell go env GOPATH)
GOBIN       := $(shell go env GOBIN)
PROTOC_GEN_GO      := $(GOPATH)/bin/protoc-gen-go
PROTOC_GEN_GO_GRPC := $(GOPATH)/bin/protoc-gen-go-grpc
MOCKGEN     := $(GOBIN)/mockgen
MOCK_DIR    := _mock

.PHONY: proto proto-install mock mock-install run build test test-unit unit-test-coverage

## mock-install: install mockgen tool
mock-install:
	go install go.uber.org/mock/mockgen@latest

## mock: regenerate all mocks from source interfaces
mock:
	@echo "Generating mocks..."
	$(MOCKGEN) \
		-source=internal/presence/repository/init.go \
		-destination=$(MOCK_DIR)/presence/mock_repository.go \
		-package=mockpresence \
		-mock_names=Presence=MockPresenceRepository
	$(MOCKGEN) \
		-source=internal/presence/usecase/init.go \
		-destination=$(MOCK_DIR)/presence/mock_usecase.go \
		-package=mockpresence \
		-mock_names=Presence=MockPresenceUsecase
	$(MOCKGEN) \
		-source=pkg/billingclient/client.go \
		-destination=$(MOCK_DIR)/pkg/billingclient/mock_billingclient.go \
		-package=mockbillingclient \
		-mock_names=BillingService=MockBillingService
	@echo "Done."

## proto-install: install protoc-gen-go and protoc-gen-go-grpc plugins
proto-install:
	go install google.golang.org/protobuf/cmd/protoc-gen-go@latest
	go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@latest

## proto: regenerate Go code from all .proto files under proto/
proto:
	@echo "Generating proto files..."
	@find $(PROTO_DIR) -name "*.proto" | while read proto_file; do \
		protoc \
			--proto_path=$(PROTO_DIR) \
			--go_out=$(GEN_DIR) \
			--go_opt=paths=source_relative \
			--go-grpc_out=$(GEN_DIR) \
			--go-grpc_opt=paths=source_relative \
			--plugin=protoc-gen-go=$(PROTOC_GEN_GO) \
			--plugin=protoc-gen-go-grpc=$(PROTOC_GEN_GO_GRPC) \
			$$(echo $$proto_file | sed 's|$(PROTO_DIR)/||'); \
	done
	@echo "Done."

mod-tidy:
	go mod tidy

run:
	go run cmd/main.go

build:
	@env CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o bin/presence cmd/main.go

test:
	go test -v ./...

test-unit:
	go test -v ./internal/presence/usecase/... ./internal/presence/handler/... ./internal/presence/repository/...

unit-test-coverage:
	go test -v -covermode=count ./... -coverprofile=coverage.cov
	go tool cover -func=coverage.cov

gen-mock-source:
	$(MOCKGEN) -package=${pkg} -destination=$(destination) -source=${source}

docker-build: build
	docker build -f Dockerfile -t presence-service:latest .

golint:
	golangci-lint run --timeout 5m --output.code-climate.path stdout
