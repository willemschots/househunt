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
