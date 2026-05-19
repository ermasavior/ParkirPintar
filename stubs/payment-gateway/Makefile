.PHONY: run build

run:
	go run cmd/main.go

build:
	@env CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o bin/payment-gateway cmd/main.go
