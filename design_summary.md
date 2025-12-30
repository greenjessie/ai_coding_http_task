# API 通知系统设计总结

## 1. 系统整体架构
- **分层架构**：采用典型的微服务架构，包括 API 接入层、后台 Worker 层和数据存储层
- **无状态设计**：API 接入层和 Worker 均为无状态，支持水平扩容
- **任务驱动**：通过数据库存储和管理通知任务，Worker 定期轮询并执行任务
- **单数据库依赖**：使用关系型数据库（如 Mysql）存储任务和尝试记录
- **最小可用拓扑**：1个 API 接入层实例 + 1个 Worker 实例 + 1个数据库实例

## 2. 核心数据模型

### NotificationTask（通知任务）
- `ID`/`task_id`：任务唯一标识符
- `PartnerID`：合作方标识，用于配额隔离和幂等去重
- `TargetURL`：目标 API 地址
- `HTTPMethod`：请求方法（GET/POST 等）
- `Headers`：请求头信息
- `Body`：请求体内容
- `IdempotencyKey`：幂等键，与 PartnerID 配合确保幂等性
- `Priority`：任务优先级
- `Status`：任务状态（pending/running/succeeded/failed/cancelled/dead）
- `NextAttemptAt`：下次尝试时间
- `MaxAttempts`：最大重试次数
- `SuccessCondition`：自定义成功判定条件（可选）
- `CreatedAt`/`UpdatedAt`：创建和更新时间

### NotificationAttempt（尝试记录）
- `ID`：尝试记录唯一标识符
- `TaskID`：关联的任务 ID
- `AttemptNumber`：尝试序号
- `StatusCode`：HTTP 响应状态码
- `LatencyMs`：请求延迟（毫秒）
- `ErrorMessage`：错误信息（如果有）
- `CreatedAt`：尝试时间

## 3. 关键业务约束

### 送达成功判定
- 默认策略：HTTP 状态码为 2xx/3xx 的响应视为送达成功
- 支持自定义成功判定条件，覆盖默认策略

### 重试策略
- 采用指数退避 + 抖动算法，避免请求风暴
- 超过最大重试次数后任务状态置为 dead
- 支持 429 状态码的特殊重试处理

### 幂等键处理
- 按 `partner_id` + `idempotency_key` 进行幂等去重
- 相同幂等键的请求不会重复创建任务

### 安全白名单
- 目标 URL 必须在白名单域名内
- 禁止内网/环回地址，防止 SSRF 攻击

### 服务等级目标 (SLO)
- 任务受理成功率：99.99%（API 层面）
- 99% 的通知在 1 小时内送达
- 95% 的通知在 5 分钟内首次尝试

## 4. 技术栈与非目标

### 技术栈
- 编程语言：Go
- 数据库：关系型数据库（如 PostgreSQL/MySQL）
- HTTP 框架：标准库或轻量级框架（如 Gin/Chi）
- 无外部 MQ 依赖，使用数据库作为任务队列

### 非目标
- 不依赖复杂的消息队列
- 不追求过度设计，简单可靠优先
- 不支持复杂的自定义成功判定逻辑（初期）
- 不支持跨地域部署（初期）

## 5. 核心 API 接口
- `POST /v1/notify`：创建通知任务
- `GET /v1/notify/{id}`：查询任务状态
- `POST /v1/notify/{id}/cancel`：取消任务（可选）

## 6. 扩展与演进
- 支持水平扩展：增加 Worker 实例提高处理能力
- 数据库扩容：垂直升级或按 partner_id/时间分片
- 可选增强：自定义成功判定、回调支持、高级认证、跨地域部署