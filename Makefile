MAIN_PACKAGE_PATH = ./cmd/server
BUILD_DIR = ./bin
BINARY_NAME = gochat
BIN_PATH = ${BUILD_DIR}/${BINARY_NAME}

all: run
fmt:
	@go fmt ./...
build: fmt
	@echo "Building the server..."
	@go build -o ${BIN_PATH} ${MAIN_PACKAGE_PATH}
	@echo "Build complete. Executable is located in ${BIN_PATH}"
	@echo "Building the frontend..."
	@cd frontend && npm install && npm run build
db:
	@echo "Starting the database container..."
	@docker run --rm -d \
		--name gochat-db \
		-e POSTGRES_USER=postgres \
		-e POSTGRES_PASSWORD=postgres \
		-e POSTGRES_DB=postgres \
		-p 5432:5432 \
		-v gochat-db-data:/var/lib/postgresql/data \
		postgres:latest && \
		echo "Waiting for the database to be ready..."
		@until docker exec gochat-db pg_isready -U postgres; do \
			sleep 1; \
		done
		@echo "Database container started successfully."
db/stop:
	@if docker ps -a --format '{{.Names}}' | grep -q '^gochat-db$$'; then \
		docker stop gochat-db; \
		else \
		echo "DB container is not running"; \
	fi
run: build db/stop db
	${BIN_PATH} -allowed-origins=http://localhost:8000 -dev
clean:
	@echo "Cleaning up build artifacts..."
	@rm -rf ${BUILD_DIR}
	@rm -rf ./frontend/build
test:
	go test -v -race ./...
test/cover:
	go test -v -race -coverprofile=/tmp/coverage.out ./...
	go tool cover -html=/tmp/coverage.out
