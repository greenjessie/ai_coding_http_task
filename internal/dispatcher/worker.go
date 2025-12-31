package dispatcher

import (
	"context"
	"encoding/json"
	"strings"
	"time"

	"api-notify/internal/config"
	"api-notify/internal/core"
	"api-notify/internal/store"
	"api-notify/pkg/httpclient"
	"api-notify/pkg/logging"
)

// Worker 通知派发Worker
// 负责定期从数据库获取待处理的通知任务并发送
// 记录发送尝试结果并处理重试逻辑

type Worker struct {
	logger    *logging.Logger
	store     *store.Store
	httpClient *httpclient.Client
	config    *config.Config
	stopCh    chan struct{}
	// Sub-struct for configuration
	settings struct {
		ConcurrentWorkers int
		Interval          time.Duration
		BatchSize         int
		RetryBackoff      time.Duration
		SensitiveHeaders  map[string]string
	}
}

// NewWorker 创建新的Worker实例

func NewWorker(logger *logging.Logger, store *store.Store, httpClient *httpclient.Client, config *config.Config) *Worker {
	worker := &Worker{
		logger:     logger,
		store:      store,
		httpClient: httpClient,
		config:     config,
		stopCh:     make(chan struct{}),
	}
	
	// Populate configuration sub-struct
	worker.settings.ConcurrentWorkers = config.Worker.Concurrency
	worker.settings.Interval = config.Worker.PollInterval
	worker.settings.BatchSize = 100 // Default batch size
	worker.settings.RetryBackoff = 5 * time.Second // Default retry backoff
	worker.settings.SensitiveHeaders = config.Security.SensitiveHeaders
	
	return worker
}

// Start 启动Worker

func (w *Worker) Start(ctx context.Context) {
	// 创建指定数量的并发Worker
	for i := 0; i < w.settings.ConcurrentWorkers; i++ {
		go w.runWorker(ctx, i)
	}
	
	w.logger.Info("Dispatcher workers started with %d concurrent workers", w.settings.ConcurrentWorkers)
}

// Stop 停止Worker

func (w *Worker) Stop() {
	close(w.stopCh)
	w.logger.Info("Dispatcher workers stopping...")
}

// runWorker 运行单个Worker实例

func (w *Worker) runWorker(ctx context.Context, id int) {
	w.logger.Debug("Worker %d started", id)
	defer w.logger.Debug("Worker %d stopped", id)

	ticker := time.NewTicker(w.settings.Interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-w.stopCh:
			return
		case <-ticker.C:
			w.processTasks(ctx)
		}
	}
}

// processTasks 处理一批任务

func (w *Worker) processTasks(ctx context.Context) {
	// 获取待处理的任务
	tasks, err := w.store.GetPendingTasks(ctx, w.settings.BatchSize)
	if err != nil {
		w.logger.Error("Failed to get pending tasks: %v", err)
		return
	}

	if len(tasks) == 0 {
		w.logger.Debug("No pending tasks found")
		return
	}

	w.logger.Info("Found %d pending tasks to process", len(tasks))

	// 逐个处理任务
	for _, task := range tasks {
		w.processTask(ctx, task)
	}
}

// processTask 处理单个任务
func (w *Worker) processTask(ctx context.Context, task *core.NotificationTask) {
	// 获取当前尝试次数
	attemptCount, err := w.store.GetAttemptCount(ctx, task.TaskID)
	if err != nil {
		w.logger.Error("Failed to get attempt count for task %s: %v", task.TaskID, err)
		return
	}

	// 记录尝试
	attempt := &core.NotificationAttempt{
		TaskID:     task.TaskID, // 使用task.TaskID而不是task.ID
		AttemptNo:  attemptCount + 1, // 尝试次数自增
		Status:     core.AttemptStatusPending,
		CreatedAt:  time.Now(),
	}

	// 发送通知并记录延迟
	startTime := time.Now()
	success, responseCode, _, err := w.sendNotification(ctx, task) // 使用 _ 忽略 responseBody
	latency := time.Since(startTime)

	if err != nil {
		w.logger.Error("Failed to send notification for task %s: %v", task.TaskID, err)
		attempt.ErrorMessage = err.Error()
		// 设置通用错误码
		attempt.ErrorCode = "HTTP_REQUEST_FAILED"
		if err.Error() == "context deadline exceeded" {
			attempt.ErrorCode = "HTTP_REQUEST_TIMEOUT"
		}
	}

	// 更新尝试记录
	attempt.Status = core.AttemptStatusSent
	attempt.HTTPStatusCode = responseCode
	attempt.LatencyMs = latency.Milliseconds()

	// 记录尝试
	if err := w.store.RecordAttempt(ctx, attempt); err != nil {
		w.logger.Error("Failed to record attempt for task %s: %v", task.TaskID, err)
		return
	}

	// 处理发送结果
	if success {
		// 发送成功，更新任务状态为已成功
		if err := w.store.UpdateTaskStatus(ctx, task.TaskID, core.TaskStatusSucceeded, time.Now()); err != nil {
			w.logger.Error("Failed to update task status to completed for task %s: %v", task.TaskID, err)
			return
		}
		w.logger.Info("Notification sent successfully for task %s, status code: %d, latency: %dms", task.TaskID, responseCode, attempt.LatencyMs)
		return
	}

	// 发送失败，处理重试逻辑
	if attemptCount+1 < task.MaxAttempts {
		// 计算下次重试时间（指数退避 + 抖动）
		nextAttemptAt := calculateNextAttempt(attemptCount, w.settings.RetryBackoff)
		// 更新任务状态为failed，设置下次尝试时间
		if err := w.store.UpdateTaskRetry(ctx, task.TaskID, attemptCount+1, nextAttemptAt); err != nil {
			w.logger.Error("Failed to update task retry for task %s: %v", task.TaskID, err)
			return
		}
		w.logger.Info("Notification failed for task %s, will retry at %s (attempt %d/%d)", task.TaskID, nextAttemptAt.Format(time.RFC3339), attemptCount+1, task.MaxAttempts)
	} else {
		// 达到最大重试次数，更新任务状态为dead
		if err := w.store.UpdateTaskStatus(ctx, task.TaskID, core.TaskStatusDead, time.Now()); err != nil {
			w.logger.Error("Failed to update task status to dead for task %s: %v", task.TaskID, err)
			return
		}
		w.logger.Info("Notification failed for task %s after %d attempts, marked as dead", task.TaskID, task.MaxAttempts)
	}
}

// sendNotification 发送单个通知
func (w *Worker) sendNotification(ctx context.Context, task *core.NotificationTask) (bool, int, string, error) {
	// 解析请求头
	var headers map[string]string
	if task.Headers != "" {
		if err := json.Unmarshal([]byte(task.Headers), &headers); err != nil {
			w.logger.Error("Failed to parse headers for task %s: %v", task.TaskID, err)
			headers = make(map[string]string)
		}
	} else {
		headers = make(map[string]string)
	}

	// 替换敏感头占位符
	for key, value := range headers {
		// 检查是否是敏感头占位符格式 {{HEADER_NAME}}
		if strings.HasPrefix(value, "{{") && strings.HasSuffix(value, "}}") {
			headerName := strings.TrimSpace(value[2 : len(value)-2])
			// 从配置中获取真实的敏感头值
			if realValue, exists := w.settings.SensitiveHeaders[headerName]; exists {
				headers[key] = realValue
				w.logger.Debug("Replaced sensitive header placeholder for task %s: %s", task.TaskID, key)
			} else {
				w.logger.Warn("Sensitive header placeholder not found in config for task %s: %s", task.TaskID, headerName)
				// 如果没有找到真实值，可以考虑移除这个头或者保留占位符
				// 这里我们选择保留占位符
			}
		}
	}

	// 创建HTTP请求
	resp, err := w.httpClient.Do(ctx, task.HTTPMethod, task.TargetURL, headers, []byte(task.Body))
	if err != nil {
		return false, 0, "", err
	}

	// 记录日志（脱敏与截断）
	w.logHTTPRequest(task, headers)

	// 根据响应码判断是否成功
	// 默认规则：2xx/3xx成功，其他错误或网络超时算失败
	// 429特殊处理（预留重试逻辑）
	success := false
	if resp.StatusCode >= 200 && resp.StatusCode < 400 {
		success = true
	} else if resp.StatusCode == 429 {
		// 429 Too Many Requests，特殊处理：需要重试
		success = false
	} else {
		// 其他状态码视为失败
		success = false
	}

	return success, resp.StatusCode, string(resp.Body), nil
}

// logHTTPRequest 记录HTTP请求日志（脱敏与截断）
func (w *Worker) logHTTPRequest(task *core.NotificationTask, headers map[string]string) {
	// 截断请求体（最长100字符）
	bodyLog := task.Body
	if len(bodyLog) > 100 {
		bodyLog = bodyLog[:100] + "..."
	}

	// 脱敏敏感头
	sanitizedHeaders := make(map[string]string)
	for k, v := range headers {
		if isSensitiveHeader(k) {
			sanitizedHeaders[k] = "[REDACTED]"
		} else {
			sanitizedHeaders[k] = v
		}
	}

	// 记录日志
	w.logger.Debug("Sending HTTP request for task %s: method=%s, url=%s, headers=%v, body=%s",
		task.TaskID, task.HTTPMethod, task.TargetURL, sanitizedHeaders, bodyLog)
}

// isSensitiveHeader 检查是否为敏感头
func isSensitiveHeader(key string) bool {
	sensitiveHeaders := map[string]bool{
		"Authorization": true,
		"Cookie":        true,
		"Set-Cookie":    true,
		"X-Auth-Token":  true,
		"Api-Key":       true,
		"Token":         true,
	}
	return sensitiveHeaders[key]
}

// calculateNextAttempt 计算下次重试时间
// 使用指数退避 + 抖动策略
func calculateNextAttempt(attemptCount int, baseBackoff time.Duration) time.Time {
	// 指数退避: baseBackoff * (2^attemptCount)
	backoff := baseBackoff
	for i := 0; i < attemptCount; i++ {
		backoff *= 2
		// 限制最大退避时间（防止无限增长）
		if backoff > 24*time.Hour {
			backoff = 24 * time.Hour
			break
		}
	}

	// 添加抖动（±10%）
	jitter := backoff / 10
	if jitter > 0 {
		// 生成随机抖动值
		randomJitter := time.Duration((int64(time.Now().UnixNano()) % int64(jitter*2)) - int64(jitter))
		backoff += randomJitter
	}

	return time.Now().Add(backoff)
}