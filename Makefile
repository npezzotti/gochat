# go source files, ignore vendor directory
SRC = $(shell find . -type f -name '*.go')

fmt:
	go fmt $(SRC)
build:
<<<<<<< HEAD
	go build -o bin/gochat .
clean:
	rm -rf bin/**
=======
	go build -o bin/go-chatroom .
clean:
	rm -rf build/**
>>>>>>> 543683956304d811e91788a11550855c929c0c7a
