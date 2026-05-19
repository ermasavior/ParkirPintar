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

mock:
	@echo "Generating mocks..."
	$(MOCKGEN) \
		-source=internal/payment/repository/init.go \
		-destination=$(MOCK_DIR)/payment/mock_repository.go \
		-package=mockpayment \
		-mock_names=Payment=MockPaymentRepository
	$(MOCKGEN) \
		-source=internal/payment/usecase/init.go \
		-destination=$(MOCK_DIR)/payment/mock_usecase.go \
		-package=mockpayment \
		-mock_names=Payment=MockPaymentUsecase
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
	@env CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o bin/payment cmd/main.go

test:
	go test -v ./...

test-unit:
	go test -v ./internal/payment/usecase/... ./internal/payment/handler/... ./internal/payment/repository/... ./internal/payment/webhook/...

unit-test-coverage:
	go test -v -covermode=count ./... -coverprofile=coverage.cov
	go tool cover -func=coverage.cov

gen-mock-source:
	$(MOCKGEN) -package=${pkg} -destination=$(destination) -source=${source}

docker-build: build
	docker build -f Dockerfile -t payment-service:latest .

golint:
	golangci-lint run --timeout 5m --output.code-climate.path stdout
