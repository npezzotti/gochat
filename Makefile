# go source files, ignore vendor directory
SRC = $(shell find . -type f -name '*.go')

fmt:
	go fmt $(SRC)
build:
	go build -o bin/gochat cmd/server/main.go
clean:
	rm -rf bin/**
test:
	go test -v ./...
