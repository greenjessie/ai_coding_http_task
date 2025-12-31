# 项目名称
PROJECT_NAME = api-notify

# 可执行文件名称
EXECUTABLE = $(PROJECT_NAME)

# Go命令
GO = go

# 配置文件路径
CONFIG_FILE = config.json

# 编译目标
.PHONY: build run migrate clean

# 默认目标
default: build

# 编译项目
build:
	@echo "Building $(PROJECT_NAME)..."
	@$(GO) build -o $(EXECUTABLE) ./cmd/$(PROJECT_NAME)

# 本地启动服务
run:
	@echo "Starting $(PROJECT_NAME) service..."
	@if [ ! -f "$(CONFIG_FILE)" ]; then \
		cp config.example.json $(CONFIG_FILE); \
		echo "Created config.json from config.example.json. Please update it with your settings."; \
	fi
	@$(GO) run ./cmd/$(PROJECT_NAME)

# 执行数据库迁移
migrate:
	@echo "Running database migration..."
	@if [ ! -f "$(CONFIG_FILE)" ]; then \
		cp config.example.json $(CONFIG_FILE); \
		echo "Created config.json from config.example.json. Please update it with your database settings."; \
	fi
	@$(GO) run ./cmd/$(PROJECT_NAME) migrate

# 清理编译文件
clean:
	@echo "Cleaning up..."
	@rm -f $(EXECUTABLE)
	@if [ -f "$(CONFIG_FILE)" ]; then rm -f $(CONFIG_FILE); fi