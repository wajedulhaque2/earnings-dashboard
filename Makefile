.PHONY: dev test build fmt lint vet migrate migrate-down migrate-status worker schedule integration check

dev:
	go run ./cmd/server
test:
	go test ./...
build:
	go build -o bin/server ./cmd/server
	go build -o bin/migrate ./cmd/migrate
	go build -o bin/worker ./cmd/worker
fmt:
	gofmt -w .
lint: vet
vet:
	go vet ./...
migrate:
	go run ./cmd/migrate up
migrate-down:
	go run ./cmd/migrate down
migrate-status:
	go run ./cmd/migrate status
worker:
	go run ./cmd/worker

schedule:
	go run ./cmd/worker schedule

integration:
	go test -tags integration ./internal/repository -v
check:
	gofmt -w .
	go vet ./...
	go test ./...
	go build ./...
