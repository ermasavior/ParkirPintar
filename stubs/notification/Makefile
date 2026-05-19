.PHONY: run build mod-tidy

run:
	go run cmd/main.go

build:
	@env CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o bin/notification cmd/main.go

mod-tidy:
	go mod tidy
