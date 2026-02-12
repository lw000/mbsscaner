# Makefile for MBS Scanner

.PHONY: build clean test fmt vet lint run help install-deps

# 变量定义
APP_NAME = mbsscaner
MAIN_FILE = main.go
BUILD_DIR = build
VERSION = 1.0.0

# 默认目标
all: clean fmt vet build

# 构建应用
build:
	@echo "Building $(APP_NAME)..."
	@if [ ! -d $(BUILD_DIR) ]; then mkdir $(BUILD_DIR); fi
	go build -o $(BUILD_DIR)/$(APP_NAME) $(MAIN_FILE)
	@echo "Build completed: $(BUILD_DIR)/$(APP_NAME)"

# 构建不同平台的版本
build-all:
	@echo "Building for multiple platforms..."
	@if [ ! -d $(BUILD_DIR) ]; then mkdir $(BUILD_DIR); fi
	
	# Windows
	GOOS=windows GOARCH=amd64 go build -o $(BUILD_DIR)/$(APP_NAME)-windows-amd64.exe $(MAIN_FILE)
	
	# Linux
	GOOS=linux GOARCH=amd64 go build -o $(BUILD_DIR)/$(APP_NAME)-linux-amd64 $(MAIN_FILE)
	
	# macOS
	GOOS=darwin GOARCH=amd64 go build -o $(BUILD_DIR)/$(APP_NAME)-darwin-amd64 $(MAIN_FILE)
	
	@echo "Multi-platform build completed"

# 运行应用
run:
	@echo "Running $(APP_NAME)..."
	go run $(MAIN_FILE) -config config.toml

# 安装依赖
install-deps:
	@echo "Installing dependencies..."
	go mod download
	go mod tidy

# 代码格式化
fmt:
	@echo "Formatting code..."
	go fmt ./...

# 代码检查
vet:
	@echo "Vetting code..."
	go vet ./...

# 代码静态分析（需要安装golangci-lint）
lint:
	@echo "Linting code..."
	@if command -v golangci-lint >/dev/null 2>&1; then \
		golangci-lint run; \
	else \
		echo "golangci-lint not found, installing..."; \
		go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest; \
		golangci-lint run; \
	fi

# 运行测试
test:
	@echo "Running tests..."
	go test -v ./...

# 测试覆盖率
test-coverage:
	@echo "Running tests with coverage..."
	go test -v -coverprofile=coverage.out ./...
	go tool cover -html=coverage.out -o coverage.html

# 清理构建文件
clean:
	@echo "Cleaning build files..."
	rm -rf $(BUILD_DIR)
	rm -f coverage.out coverage.html

# 交叉编译发布版本
release:
	@echo "Building release version $(VERSION)..."
	@if [ ! -d $(BUILD_DIR) ]; then mkdir $(BUILD_DIR); fi
	
	# 构建发布版本（禁用调试信息）
	go build -ldflags="-s -w" -o $(BUILD_DIR)/$(APP_NAME)-v$(VERSION) $(MAIN_FILE)
	@echo "Release build completed: $(BUILD_DIR)/$(APP_NAME)-v$(VERSION)"

# 创建配置文件
setup-config:
	@echo "Setting up configuration..."
	@if [ ! -f config.toml ]; then \
		cp config.toml.example config.toml; \
		echo "Created config.toml from example"; \
	else \
		echo "config.toml already exists"; \
	fi

# 创建日志目录
setup-logs:
	@echo "Creating logs directory..."
	@if [ ! -d logs ]; then \
		mkdir logs; \
		echo "Created logs directory"; \
	else \
		echo "logs directory already exists"; \
	fi

# 完整环境设置
setup: setup-config setup-logs install-deps
	@echo "Environment setup completed"

# 显示帮助
help:
	@echo "Available targets:"
	@echo "  build          - Build the application"
	@echo "  build-all      - Build for multiple platforms"
	@echo "  run            - Run the application"
	@echo "  install-deps   - Install dependencies"
	@echo "  fmt            - Format code"
	@echo "  vet            - Vet code"
	@echo "  lint           - Run static analysis"
	@echo "  test           - Run tests"
	@echo "  test-coverage  - Run tests with coverage"
	@echo "  clean          - Clean build files"
	@echo "  release        - Build release version"
	@echo "  setup-config   - Create config file from example"
	@echo "  setup-logs     - Create logs directory"
	@echo "  setup          - Complete environment setup"
	@echo "  help           - Show this help message"
	@echo "  all            - Clean, format, vet, and build"