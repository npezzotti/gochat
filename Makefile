# go source files, ignore vendor directory
SRC = $(shell find . -type f -name '*.go')

fmt:
	go fmt $(SRC)
build:
	go build -o bin/go-chatroom .
clean:
	rm -rf build/**
