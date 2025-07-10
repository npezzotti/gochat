MAIN_PACKAGE_PATH = ./cmd/server
BINARY_NAME = gochat

.PHONY: fmt run db build clean test test/cover

fmt:
	go fmt ./...
build: fmt
	go build -o bin/${BINARY_NAME} ${MAIN_PACKAGE_PATH}
	cd frontend && npm install && npm run build
db:
	docker run --rm -d \
		--name gochat-db \
		-e POSTGRES_USER=postgres \
		-e POSTGRES_PASSWORD=postgres \
		-e POSTGRES_DB=postgres \
		-p 5432:5432 \
		-v gochat-db-data:/var/lib/postgresql/data \
		postgres:latest
db/stop:
	@if docker ps -a --format '{{.Names}}' | grep -q '^gochat-db$$'; then \
		docker stop gochat-db; \
	fi
run: build db/stop db
	bin/${BINARY_NAME} -allowed-origins=http://localhost:8000 -dev
clean:
	rm -rf ./bin
	rm -rf ./frontend/build
test:
	go test -v -race ./...
test/cover:
	go test -v -race -coverprofile=/tmp/coverage.out ./...
	go tool cover -html=/tmp/coverage.out
