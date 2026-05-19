PROTO_DIR   := proto
GEN_DIR     := gen
GOPATH      := $(shell go env GOPATH)
GOBIN       := $(shell go env GOBIN)
PROTOC_GEN_GO      := $(GOPATH)/bin/protoc-gen-go
PROTOC_GEN_GO_GRPC := $(GOPATH)/bin/protoc-gen-go-grpc
MOCKGEN     := $(GOBIN)/mockgen
MOCK_DIR    := _mock

.PHONY: proto proto-install mock mock-install run build test test-unit unit-test-coverage

mock-install:
	go install go.uber.org/mock/mockgen@latest

## mock: regenerate all mocks from source interfaces
mock:
	@echo "Generating mocks..."
	$(MOCKGEN) \
		-source=internal/billing/repository/init.go \
		-destination=$(MOCK_DIR)/billing/mock_repository.go \
		-package=mockbilling \
		-mock_names=Billing=MockBillingRepository
	$(MOCKGEN) \
		-source=internal/billing/usecase/init.go \
		-destination=$(MOCK_DIR)/billing/mock_usecase.go \
		-package=mockbilling \
		-mock_names=Billing=MockBillingUsecase
	$(MOCKGEN) \
		-source=pkg/paymentclient/client.go \
		-destination=$(MOCK_DIR)/pkg/paymentclient/mock_paymentclient.go \
		-package=mockpaymentclient \
		-mock_names=PaymentService=MockPaymentService
	@echo "Done."

proto-install:
	go install google.golang.org/protobuf/cmd/protoc-gen-go@latest
	go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@latest

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
	@env CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o bin/billing cmd/main.go

test:
	go test -v ./...

test-unit:
	go test -v ./internal/billing/usecase/... ./internal/billing/handler/... ./internal/billing/repository/... ./pkg/pricing/...

unit-test-coverage:
	go test -v -covermode=count ./... -coverprofile=coverage.cov
	go tool cover -func=coverage.cov

gen-mock-source:
	$(MOCKGEN) -package=${pkg} -destination=$(destination) -source=${source}

docker-build: build
	docker build -f Dockerfile -t billing-service:latest .

golint:
	golangci-lint run --timeout 5m --output.code-climate.path stdout
