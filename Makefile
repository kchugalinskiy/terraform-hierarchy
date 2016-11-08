all: build

build:
	CGO_ENABLED=0 go build -tags netgo -a -o hierarchy

test-short:
	go test --short ./...