### Prompt 0
你现在扮演一名资深 Go 后端工程师，帮助我实现一个“API 通知系统”（对内提供 HTTP API，对外调用各类合作方的 HTTP(S) API，保证请求可靠发出，业务方不关心返回值）。

这是该系统的技术设计文档，请先通读并理解整体架构与约束：API design.docx
阅读完成后，请用 5～10 条要点总结：

系统整体架构
核心数据模型（NotificationTask / NotificationAttempt）
关键业务约束（送达成功判定、重试策略、幂等键处理、安全白名单等）
技术栈与非目标（比如不依赖 MQ、简单可靠优先）
总结完后等待我下一步指令，不要直接开始写代码。


### Prompt 1
帮我基于上面的技术设计，用 Go 创建一个最小可用但结构清晰的项目骨架，不要引入过多第三方库，简单可靠优先。

要求：

使用 Go modules，在当前目录下初始化项目内容。
采用分层结构，大致目录如下（如有更合理调整可以简要解释）：
cmd/api-notify/：服务入口 main.go
internal/config/：配置加载（从 env / yaml）
internal/httpapi/：HTTP 接入层（路由、handler）
internal/store/：数据库访问与表模型
internal/dispatcher/：通知派发与重试 worker
internal/core/：核心业务逻辑与领域模型（NotificationTask 等）
pkg/logging/：日志封装
pkg/httpclient/：统一 HTTP 客户端封装
在 main.go 中串起：配置加载、DB 初始化、HTTP server 启动、后台 worker 启动与优雅退出（context + signal）。
go.mod、go.sum 一并生成，数据库默认用 MySQL。
请直接输出完整的目录结构与关键文件的初始代码（可以是最小可运行版本，函数体允许留少量 TODO 注释）。

### Prompt 2：数据模型与数据库表设计
在现有项目骨架基础上，实现通知任务相关的数据模型与数据库表。

目标：

数据模型：
NotificationTask：包含 id、target_url、http_method、headers、body、partner_id、priority、status（pending/running/succeeded/failed/dead）、idempotency_key、next_attempt_at、max_attempts、created_at、updated_at 等字段。
NotificationAttempt：包含 id、task_id、attempt_no、status、http_status_code、error_code、error_message、latency_ms、created_at 等字段。
表结构与索引：
写出建表 SQL（自动迁移或手写 SQL 皆可，但给出清晰版本）。
关键索引：按 status + next_attempt_at 查询未完成任务；按 idempotency_key + partner_id 做幂等去重；按 task_id 查尝试记录。
要求：

在 internal/core 中定义领域模型（Go struct），不要把 JSON/DB tag 和业务 struct 混得太乱，可适度集中在 store 层。
在 internal/store 中实现任务与尝试记录的基础 CRUD，特别是：
创建任务时设置初始状态 pending 与 next_attempt_at=now()。
按幂等键与 partner_id 查重的函数。
输出：
Go struct 定义
建表 SQL（或 migration）
关键仓储接口与实现示例。

### Prompt 3：实现 HTTP API 接入层（POST /v1/notify, GET /v1/notify/{id}）
在现有项目基础上，实现对内标准 HTTP API：

POST /v1/notify：创建通知任务
请求体字段（JSON）：
target_url (string, 必填)
method (string, 可选，默认 POST)
headers (map[string]string, 可选)
body (任意 JSON，内部可存为 json.RawMessage 或 []byte)
idempotency_key (string, 可选)
partner_id (string, 必填，用于幂等与配额隔离)
priority (int, 可选，默认值即可)
success_condition (可选字符串，暂时先支持“默认 2xx/3xx 成功”，字段先预留即可)。
返回体：
task_id
status （创建即 pending）
行为：
校验目标 URL 在白名单域名内（后续可以从配置中读取，先给出接口预留）。
支持 Idempotency-Key HTTP 头或请求体字段两种幂等键来源。
若同一 partner_id + idempotency_key 已存在任务，则返回已有 task_id，不再新建。
GET /v1/notify/{id}：查询任务状态
返回任务状态、最近一次尝试的摘要（HTTP 状态码、错误码、最近一次尝试时间等）。
可选：预留取消接口 POST /v1/notify/{id}/cancel，实现可以先简单处理：仅允许在非终态（pending/running）时改为 dead。
技术要求：

使用标准库 net/http 或常用路由库（如 chi/gin 等，你选一个简单的即可）。
参数校验、错误返回格式统一（包含错误码与错误信息）。
不在 handler 中写复杂逻辑，通过 internal/core 的服务层来完成。
请输出完整的 handler 代码示例、路由注册、以及必要的请求/响应 struct。

### Prompt 4：实现派发 Worker 与重试逻辑
现在实现后台派发与重试逻辑，目标是“简单可靠”，不要搞复杂的调度框架。

需求：

Worker 逻辑：
周期性从 DB 中拉取状态为 pending 或 failed 且 next_attempt_at <= now() 的任务。
每次拉取时使用行级锁避免多实例重复消费：
若你选的是 Postgres，用 SELECT ... FOR UPDATE SKIP LOCKED。
若是 MySQL，可用 SELECT ... FOR UPDATE + 简单分片/限制并发的方式，尽量减少锁冲突。
取到任务后：
构造 HTTP 请求（method/headers/body），通过统一 pkg/httpclient 发送。
按设计文档中的规则判定成功：默认 2xx/3xx 成功，其它错误或网络超时算失败；预留 429 特殊重试。
记录 NotificationAttempt，更新 NotificationTask 状态和 next_attempt_at。
重试策略：
指数退避 + 抖动（例如：base = 30s，指数增长，支持最大重试次数）。
超过最大重试次数后状态置为 dead。
HTTP 客户端：
配置合理的超时时间（比如 3～10s），限制最大连接数。
对请求与响应内容的日志做脱敏与截断，不要记录完整 body。
请实现：

internal/dispatcher/worker.go 的核心逻辑：拉取任务、派发、写入 attempt、计算下一次重试时间。
与 store 层的接口：拿任务、更新任务状态、插入尝试记录。
关键函数给出完整代码，必要的辅助函数用 TODO 说明。

### Prompt 5：配置、安全白名单、日志与指标
在现有实现上补齐配置管理、安全与观测性，保持简单可靠：

配置：
在 internal/config 中增加结构体字段：
DB 连接信息
HTTP server 端口
目标域名白名单（可以是逗号分隔字符串或列表）
每个 partner 的速率限制与最大并发（先用简单的全局/按 partner 限速结构，后续可扩展）。
支持从环境变量 / 配置文件加载。
安全：
在创建任务与派发前，对 target_url 做域名白名单校验，禁止内网/环回地址，防止 SSRF。
在存储 header 模板时，对敏感字段（如 Authorization）只存占位符，真正密钥从配置或未来 KMS 集成中获取（现在可以用接口预留+TODO 说明）。
日志与指标：
使用一个轻量级日志库（或标准库 log）统一输出字段：task_id、partner_id、attempt_no、status、http_status_code 等。
预留指标采集点（例如用 Prometheus client 或你建议的简单方案）：
入站 QPS
派发成功率
平均延迟
平均重试次数
dead 任务比例
请补充和修改相应代码，给出关键部分的实现示例，并说明如何通过配置文件/环境变量启动服务。

### Prompt 6：最小可运行 Demo 与本地验证说明
在现有项目基础上，帮我整理一个“最小可运行 Demo”和本地验证步骤，便于在 Trae / 本地环境启动测试：

增加简单的 Makefile 或 task.sh，支持：
make run：本地启动服务
make migrate：执行数据库建表迁移
给出一个本地 docker-compose 示例（可选）：包含 DB 与本服务，方便一键启动。
写一份简短的 README（可以放在项目根目录）：
如何配置环境变量
如何启动服务
如何用 curl 或 Postman 调用 /v1/notify 并在日志/DB 中看到任务与重试。
请直接输出 Makefile、docker-compose.yml 与 README 的示例内容。

如果你觉得某一步代码量太大，可以把该 Prompt 再拆成两个更小的 Prompt 给 Trae 使用，我也可以根据你现有的项目情况帮你再微调这些 Prompt。

