.PHONY: test build

test:
	go test ./...

build:
	go build -o dist/diagoram ./cmd/diagoram
