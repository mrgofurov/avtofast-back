.PHONY: build test test-race benchmark run migrate-up migrate-down seed

build:
	@mkdir -p bin
	go build -o bin/api ./cmd/api
	go build -o bin/migrate ./cmd/migrate
	go build -o bin/seed ./cmd/seed

test:
	go test -v ./...

test-race:
	go test -v -race ./...

benchmark:
	go test -bench=. -benchmem ./test/benchmark/...

run: build
	./bin/api

migrate-up:
	go run ./cmd/migrate/main.go -dir=up

migrate-down:
	go run ./cmd/migrate/main.go -dir=down

seed:
	go run ./cmd/seed/main.go
