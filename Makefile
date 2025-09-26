.PHONY: help build run test lint fmt vet clean dev install-deps

# 应用程序名称
APP_NAME := z-tavern-backend
BUILD_DIR := ./bin
CMD_DIR := ./cmd/api

# Go 相关设置
GO := go
GOFLAGS := -ldflags="-s -w"

# 默认目标
help: ## 显示帮助信息
	@echo "Z Tavern Backend Makefile Commands:"
	@echo ""
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | sort | awk 'BEGIN {FS = ":.*?## "}; {printf "  \033[36m%-15s\033[0m %s\n", $$1, $$2}'
	@echo ""

build: ## 编译应用程序
	@echo "Building $(APP_NAME)..."
	@mkdir -p $(BUILD_DIR)
	$(GO) build $(GOFLAGS) -o $(BUILD_DIR)/$(APP_NAME) $(CMD_DIR)
	@echo "Build complete: $(BUILD_DIR)/$(APP_NAME)"

run: ## 运行应用程序 (开发模式)
	@echo "Running $(APP_NAME) in development mode..."
	$(GO) run $(CMD_DIR)

dev: ## 开发模式运行 (热重载需要额外工具)
	@echo "Starting development server..."
	$(GO) run $(CMD_DIR)

test: ## 运行所有测试
	@echo "Running tests..."
	$(GO) test ./... -v

test-coverage: ## 运行测试并生成覆盖率报告
	@echo "Running tests with coverage..."
	$(GO) test ./... -coverprofile=coverage.out
	$(GO) tool cover -html=coverage.out -o coverage.html
	@echo "Coverage report generated: coverage.html"

bench: ## 运行性能测试
	@echo "Running benchmarks..."
	$(GO) test ./... -bench=. -benchmem

race: ## 运行竞态检测测试
	@echo "Running race detection tests..."
	$(GO) test ./... -race

lint: ## 运行代码检查
	@echo "Running linter..."
	@if command -v golangci-lint >/dev/null 2>&1; then \
		golangci-lint run; \
	else \
		echo "golangci-lint not found, running go vet instead"; \
		$(GO) vet ./...; \
	fi

fmt: ## 格式化代码
	@echo "Formatting code..."
	$(GO) fmt ./...

vet: ## 运行 go vet
	@echo "Running go vet..."
	$(GO) vet ./...

clean: ## 清理编译产物
	@echo "Cleaning build artifacts..."
	@rm -rf $(BUILD_DIR)
	@rm -f coverage.out coverage.html
	@echo "Clean complete"

install-deps: ## 安装项目依赖
	@echo "Installing dependencies..."
	$(GO) mod download
	$(GO) mod tidy

update-deps: ## 更新项目依赖
	@echo "Updating dependencies..."
	$(GO) get -u ./...
	$(GO) mod tidy

docker-build: ## 构建 Docker 镜像
	@echo "Building Docker image..."
	docker build -t $(APP_NAME) .

docker-run: ## 运行 Docker 容器
	@echo "Running Docker container..."
	docker run -p 8080:8080 --rm $(APP_NAME)

# 开发环境设置
.env:
	@echo "Creating .env file..."
	@echo "PORT=8080" > .env
	@echo ".env file created with default values"

# 全面检查 (CI/CD 使用)
ci: fmt vet lint test race ## 运行所有 CI 检查

# 快速检查
check: fmt vet test ## 快速代码检查