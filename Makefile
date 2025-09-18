MAIN_PACKAGE_PATH = ./cmd/server
BUILD_DIR = ./bin
BINARY_NAME = gochat
BIN_PATH = ${BUILD_DIR}/${BINARY_NAME}
FRONTEND_DIR = ./frontend
PACKER_DIR = ./packer
TERRAFORM_DIR = ./terraform
DEV_BACKEND_ADDR = localhost:8000

all: run
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
.PHONY: db
.PHONY: run
run: db/stop db
	@$(MAKE) -j2 run/server run/frontend
.PHONY: go/fmt
go/fmt:
	@echo "Formatting Go code..."
	@go fmt ./...
.PHONY: packer/fmt
packer/fmt: packer/init
	@echo "Formatting Packer configuration..."
	@pushd ${PACKER_DIR}; packer fmt gochat.pkr.hcl; popd
.PHONY: terraform/fmt
terraform/fmt: terraform/init
	@echo "Formatting Terraform configuration..."
	@pushd ${TERRAFORM_DIR}; terraform fmt; popd
.PHONY: fmt
fmt: go/fmt packer/fmt terraform/fmt
.PHONY: build/app
build/app: go/fmt fmt
	@echo "Building the server..."
	@GOOS=linux GOARCH=amd64 go build -o ${BIN_PATH} ${MAIN_PACKAGE_PATH}
	@echo "Build complete. Executable is located in ${BIN_PATH}"
	@echo "Building the frontend..."
	@cd frontend && npm install && npm run build
	@echo "Frontend build complete."
.PHONY: build
build: fmt build/app
	@echo "Building AMI with Packer..."
	@pushd ${PACKER_DIR}; packer build gochat.pkr.hcl; popd
	@echo "AMI build complete."
.PHONY: packer/init
packer/init:
	@echo "Initializing Packer..."
	@pushd ${PACKER_DIR}; packer init .; popd
	@echo "Packer initialized."
.PHONY: packer/validate
packer/validate: packer/init
	@echo "Validating Packer configuration..."
	@pushd ${PACKER_DIR}; packer init . && packer validate gochat.pkr.hcl; popd
	@echo "Packer configuration is valid."
.PHONY: terraform/init
terraform/init:
	@echo "Initializing Terraform..."
	@pushd ${TERRAFORM_DIR}; terraform init -input=false; popd
	@echo "Terraform initialized."
.PHONY: terraform/validate
terraform/validate: terraform/init
	@echo "Initializing Terraform configuration..."
	@pushd ${TERRAFORM_DIR}; terraform init && terraform validate; popd
	@echo "Terraform configuration is valid."
.PHONY: validate
validate: packer/validate terraform/validate
.PHONY: deploy
deploy: packer/init terraform/init build
	@echo "Deploying the application..."
	@pusdh terraform
	terraform plan -out=tfplan -input=false
	terraform apply -input=false tfplan
	@popd
	@echo "Deployment complete."
.PHONY: clean
clean:
	@echo "Cleaning up build artifacts..."
	@rm -rf ${BUILD_DIR}
	@rm -rf ./frontend/build
.PHONY: test
test:
	go test -v -race ./...
.PHONY: test/cover
test/cover:
	go test -v -race -coverprofile=/tmp/coverage.out ./...
	go tool cover -html=/tmp/coverage.out
