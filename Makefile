VERSION := localdev

default: test

lint:
	golangci-lint run

test:
	go test ./...

test-race:
	go test -race ./...

build: test
	docker build -t willemdev/househunt:$(VERSION) .

dump-schema:
	go run cmd/dbmigrate/*.go schema.db && sqlite3 schema.db .schema > migrations/docs/schema.gen.sql && rm schema.db
