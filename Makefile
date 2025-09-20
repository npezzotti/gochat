MAIN_PACKAGE_PATH = ./cmd/server
BUILD_DIR = ./bin
BINARY_NAME = gochat
BIN_PATH = ${BUILD_DIR}/${BINARY_NAME}
FRONTEND_DIR = ./frontend
DEV_BACKEND_ADDR = localhost:8000

all: run
.PHONY: db
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
.PHONY: db/stop
db/stop:
	@if docker ps -a --format '{{.Names}}' | grep -q '^gochat-db$$'; then \
		docker stop gochat-db; \
		else \
		echo "DB container is not running"; \
	fi
.PHONY: run/server
run/server:
	@echo "Starting the server..."
	@cd ${MAIN_PACKAGE_PATH} && go run . -addr=${DEV_BACKEND_ADDR} -allowed-origins=http://localhost:3000,http://${DEV_BACKEND_ADDR}
.PHONY: run/frontend
run/frontend:
	@echo "Starting the frontend..."
	@cd ${FRONTEND_DIR} && npm install && DANGEROUSLY_DISABLE_HOST_CHECK=true REACT_APP_WS_DEV_HOST=${DEV_BACKEND_ADDR} npm start
.PHONY: run
run: db/stop db
	@$(MAKE) -j2 run/server run/frontend
.PHONY: fmt
fmt:
	@echo "Formatting Go code..."
	@go fmt ./...
.PHONY: go/build
.PHONY: test
test:
	go test -v -race ./...
.PHONY: test/cover
test/cover:
	go test -v -race -coverprofile=/tmp/coverage.out ./...
	go tool cover -html=/tmp/coverage.out
go/build: fmt
	@echo "Building Go application..."
	@GOOS=linux GOARCH=amd64 go build -o ${BIN_PATH} ${MAIN_PACKAGE_PATH}
	@echo "Build complete. Executable is located in ${BIN_PATH}"
.PHONY: frontend/build
frontend/build: 
	@echo "Building the frontend..."
	@cd ${FRONTEND_DIR} && npm install && npm run build
	@echo "Frontend build complete."
.PHONY: build
build: go/build frontend/build
.PHONY: clean
clean:
	@echo "Cleaning up build artifacts..."
	@rm -rf ${BUILD_DIR}
	@rm -rf ${FRONTEND_DIR}/build
