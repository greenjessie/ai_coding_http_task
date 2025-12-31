# API-Notify 快速开始指南

## 功能简介
API-Notify是一个可靠的通知分发服务，支持HTTP回调、任务重试、幂等性处理等功能。

## 环境准备
- Go 1.19+ (本地开发)
- Docker & Docker Compose (容器化运行，可选)
- MySQL 8.0 (数据库)

## 配置管理

### 配置文件方式
1. 复制示例配置文件：
```bash
cp config.example.json config.json
```

2. 修改 `config.json` 中的配置：
- 数据库连接信息
- HTTP服务器端口
- Worker配置
- 安全设置（域名白名单、敏感头处理）

### 环境变量方式
可以通过环境变量覆盖配置文件中的值，格式为：`API_NOTIFY_<配置键路径>`

例如：
```bash
export API_NOTIFY_SERVER_PORT=9090
export API_NOTIFY_DATABASE_USER=myuser
export API_NOTIFY_DATABASE_PASSWORD=mypassword
export API_NOTIFY_SECURITY_ALLOWEDDOMAINS="*.example.com,api.example.org"
```

## 服务启动

### 本地启动（使用Makefile）

1. **执行数据库迁移**：
```bash
make migrate
```

2. **启动服务**：
```bash
make run
```

### Docker Compose启动

1. 确保已创建 `config.json` 并配置正确
2. 启动服务：
```bash
docker-compose up -d
```

## API测试

### 创建通知任务
使用 `curl` 测试 `/v1/notify` 端点：

```bash
curl -X POST http://localhost:8080/v1/notify \
  -H "Content-Type: application/json" \
  -d '{
    "partner_id": "test_partner",
    "target_url": "http://example.com/webhook",
    "method": "POST",
    "headers": {
      "Content-Type": "application/json",
      "Authorization": "Bearer token123"
    },
    "body": {"message": "Hello from API-Notify"}
  }'
```

**响应示例**：
```json
{
  "task_id": "task_1234567890_abcdef12",
  "status": "pending"
}
```

### 查询任务状态
```bash
curl http://localhost:8080/v1/notify/{task_id}
```

### 取消任务
```bash
curl -X POST http://localhost:8080/v1/notify/{task_id}/cancel
```

## 验证

### 查看日志
服务日志会显示：
- 接收到的通知请求
- 任务创建信息
- 任务执行状态（成功/失败/重试）
- 指标统计信息（每5秒打印一次）

### 查看数据库
检查以下表中的数据：
- `notification_tasks`: 任务基本信息
- `notification_attempts`: 任务执行尝试记录

## 核心功能验证

### 任务重试
1. 创建一个指向不存在或会返回错误的URL的通知任务
2. 在日志中观察任务的重试过程
3. 检查`notification_attempts`表中的尝试记录

### 敏感头处理
1. 创建包含敏感头（如Authorization、Cookie）的通知任务
2. 查看日志和数据库，确认敏感头已被替换为占位符

### 幂等性
1. 使用相同的`idempotency_key`创建两次通知任务
2. 验证第二次请求返回的是第一次创建的任务ID

## 常见问题

### 数据库连接失败
检查`config.json`中的数据库配置是否正确，确保MySQL服务已启动。

### 目标URL被拒绝
确保目标URL的域名在`security.allowedDomains`配置中，或设置为`["*"]`允许所有域名（仅开发环境）。

### 敏感头未解密
开发环境中，敏感头会自动保存到配置中。生产环境需要实现KMS集成来安全存储敏感信息。

## 开发与调试

### 查看指标
服务会每5秒打印一次指标统计信息，包括：
- 入站请求QPS
- 任务派发成功率
- 平均延迟
- 重试次数分布

### 代码结构
```
├── cmd/api-notify/      # 主程序入口
├── internal/core/       # 核心业务模型
├── internal/config/     # 配置管理
├── internal/dispatcher/ # 任务调度器
├── internal/httpapi/    # HTTP API
├── internal/store/      # 数据库存储
└── internal/metrics/    # 指标收集
```