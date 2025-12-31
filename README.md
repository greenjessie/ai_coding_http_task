# API-Notify 通知服务

一个简单可靠的通知派发与重试服务，支持多实例部署和高并发处理。

## 功能特性

- **配置管理**：支持从环境变量或配置文件加载配置
- **安全防护**：域名白名单校验、敏感头占位符、防SSRF攻击
- **指标监控**：入站QPS、派发成功率、平均延迟等指标采集
- **通知派发**：支持HTTP/HTTPS通知、自定义Header和Body
- **重试机制**：指数退避+抖动策略，支持最大重试次数和重试间隔配置
- **高可用**：支持多实例部署，使用MySQL行级锁保证任务不重复处理

## 配置管理

### 配置文件

创建一个`config.json`文件，参考`config.example.json`：

```json
{
  "Server": {
    "Port": 8080,
    "ReadTimeout": 10,
    "WriteTimeout": 10
  },
  "Database": {
    "DSN": "root:password@tcp(localhost:3306)/api_notify?charset=utf8mb4&parseTime=True&loc=Local",
    "MaxOpenConns": 20,
    "MaxIdleConns": 10,
    "ConnMaxLifetime": 300
  },
  "Worker": {
    "PollInterval": 5,
    "MaxAttempts": 10,
    "Concurrency": 5
  },
  "Security": {
    "AllowedDomains": ["example.com", "*.example.org"],
    "SensitiveHeaders": {
      "Authorization": "Bearer your-secret-token",
      "Api-Key": "your-api-key"
    }
  },
  "RateLimit": {
    "Global": {
      "RequestsPerSecond": 100,
      "Burst": 200
    },
    "PerPartner": {
      "RequestsPerSecond": 10,
      "Burst": 20
    }
  },
  "Log": {
    "Level": "info",
    "Format": "text"
  }
}
```

### 环境变量

可以使用环境变量覆盖配置文件中的值：

| 环境变量名 | 类型 | 描述 |
|------------|------|------|
| `SERVER_PORT` | int | HTTP服务器端口 |
| `DB_DSN` | string | 数据库连接字符串 |
| `WORKER_POLL_INTERVAL` | int | Worker轮询间隔（秒） |
| `WORKER_MAX_ATTEMPTS` | int | 最大重试次数 |
| `WORKER_CONCURRENCY` | int | 并发Worker数量 |
| `SECURITY_ALLOWED_DOMAINS` | string | 允许的目标域名（逗号分隔） |
| `LOG_LEVEL` | string | 日志级别：debug, info, warn, error |

## 安全特性

### 域名白名单校验

配置`Security.AllowedDomains`可以限制通知的目标域名，防止SSRF攻击：

```json
{
  "Security": {
    "AllowedDomains": ["example.com", "*.example.org"]
  }
}
```

### 敏感头占位符

存储通知模板时，可以使用占位符代替真实的敏感头值：

```json
{
  "headers": {
    "Authorization": "{{Authorization}}",
    "Api-Key": "{{Api-Key}}"
  }
}
```

真实的敏感头值存储在配置文件中：

```json
{
  "Security": {
    "SensitiveHeaders": {
      "Authorization": "Bearer your-secret-token",
      "Api-Key": "your-api-key"
    }
  }
}
```

## 指标监控

### 内置指标收集

服务内置了简单的内存指标收集器，定期输出以下指标：

- `InboundRequests`：入站请求数量
- `NotificationsSent`：通知发送数量
- `SuccessCount`：成功数量
- `FailureCount`：失败数量
- `AverageLatency`：平均延迟
- `AverageRetries`：平均重试次数
- `DeadTasks`：Dead任务数量

### 扩展到Prometheus

可以将`internal/metrics/metrics.go`中的`Metrics`接口实现替换为Prometheus实现，以支持更强大的监控功能。

## API端点

### 创建通知任务

```
POST /api/v1/notifications
```

请求体：

```json
{
  "partner_id": "partner-123",
  "target_url": "https://example.com/webhook",
  "method": "POST",
  "headers": {
    "Content-Type": "application/json",
    "Authorization": "{{Authorization}}"
  },
  "body": {
    "event": "order_created",
    "data": {"order_id": "12345"}
  },
  "max_attempts": 5
}
```

响应：

```json
{
  "task_id": "task-12345",
  "status": "pending"
}
```

### 获取任务状态

```
GET /api/v1/notifications/{task_id}
```

响应：

```json
{
  "task_id": "task-12345",
  "partner_id": "partner-123",
  "target_url": "https://example.com/webhook",
  "status": "succeeded",
  "attempt_count": 1,
  "created_at": "2023-05-10T12:00:00Z",
  "updated_at": "2023-05-10T12:00:05Z"
}
```

## 运行服务

### 从源码编译

```bash
go build ./cmd/api-notify
```

### 使用配置文件运行

```bash
./api-notify --config config.json
```

### 使用环境变量运行

```bash
SERVER_PORT=8080 DB_DSN="root:password@tcp(localhost:3306)/api_notify?charset=utf8mb4&parseTime=True&loc=Local" ./api-notify
```

## 数据库表结构

### 通知任务表

```sql
CREATE TABLE notification_tasks (
    id VARCHAR(64) PRIMARY KEY,
    partner_id VARCHAR(64) NOT NULL,
    target_url VARCHAR(2048) NOT NULL,
    http_method VARCHAR(10) NOT NULL,
    headers TEXT,
    body TEXT,
    max_attempts INT NOT NULL DEFAULT 5,
    status VARCHAR(20) NOT NULL DEFAULT 'pending',
    next_attempt_at DATETIME,
    created_at DATETIME NOT NULL,
    updated_at DATETIME NOT NULL,
    INDEX idx_status (status),
    INDEX idx_next_attempt_at (next_attempt_at)
);
```

### 通知尝试表

```sql
CREATE TABLE notification_attempts (
    id BIGINT AUTO_INCREMENT PRIMARY KEY,
    task_id VARCHAR(64) NOT NULL,
    attempt_no INT NOT NULL,
    status VARCHAR(20) NOT NULL,
    http_status_code INT,
    response_body TEXT,
    error_message TEXT,
    error_code VARCHAR(50),
    latency_ms BIGINT,
    created_at DATETIME NOT NULL,
    INDEX idx_task_id (task_id),
    FOREIGN KEY (task_id) REFERENCES notification_tasks(id) ON DELETE CASCADE
);
```

## 架构设计

### 多实例部署

服务支持多实例部署，通过MySQL行级锁保证任务不重复处理：

```sql
UPDATE notification_tasks SET status = 'processing', next_attempt_at = ? WHERE id = ? AND status = 'pending' AND next_attempt_at <= NOW();
```

### 重试策略

使用指数退避+抖动策略计算下次重试时间：

```go
// 指数退避: baseBackoff * (2^attemptCount)
// 添加抖动（±10%）以避免"惊群效应"
```

## 开发指南

### 项目结构

```
├── cmd/                  # 命令行入口
│   └── api-notify/       # 主程序入口
├── internal/             # 内部包
│   ├── config/           # 配置管理
│   ├── dispatcher/       # 任务派发器
│   ├── httpapi/          # HTTP API
│   ├── metrics/          # 指标收集
│   └── store/            # 数据存储
├── pkg/                  # 公共包
│   ├── httpclient/       # HTTP客户端
│   ├── logging/          # 日志
│   └── utils/            # 工具函数
├── config.example.json   # 配置示例
└── README.md             # 项目说明
```

### 测试

```bash
go test ./...
```

## 许可证

MIT