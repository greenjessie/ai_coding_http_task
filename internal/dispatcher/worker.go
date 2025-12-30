package dispatcher

import (
	"context"
	"encoding/json"
	"time"

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
	config    WorkerConfig
	stopCh    chan struct{}
}

// WorkerConfig Worker配置

type WorkerConfig struct {
	Interval       time.Duration // 轮询间隔
	BatchSize      int           // 每次处理的批次大小
	MaxRetries     int           // 最大重试次数
	RetryBackoff   time.Duration // 重试间隔基数
	ConcurrentWorkers int        // 并发Worker数量
}

// NewWorker 创建新的Worker实例

func NewWorker(logger *logging.Logger, store *store.Store, httpClient *httpclient.Client, config WorkerConfig) *Worker {
	return &Worker{
		logger:     logger,
		store:      store,
		httpClient: httpClient,
		config:     config,
		stopCh:     make(chan struct{}),
	}
}

// Start 启动Worker

func (w *Worker) Start(ctx context.Context) {
	// 创建指定数量的并发Worker
	for i := 0; i < w.config.ConcurrentWorkers; i++ {
		go w.runWorker(ctx, i)
	}
	
	w.logger.Info("Dispatcher workers started with %d concurrent workers", w.config.ConcurrentWorkers)
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

	ticker := time.NewTicker(w.config.Interval)
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
	tasks, err := w.store.GetPendingTasks(ctx, w.config.BatchSize)
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
	// 记录尝试
	attempt := &core.NotificationAttempt{
		TaskID:     task.TaskID, // 使用task.TaskID而不是task.ID
		PartnerID:  task.PartnerID,
		Status:     core.AttemptStatusPending,
		CreatedAt:  time.Now(),
		UpdatedAt:  time.Now(),
	}

	// 发送通知
	success, responseCode, responseBody, err := w.sendNotification(ctx, task)
	if err != nil {
		w.logger.Error("Failed to send notification for task %s: %v", task.TaskID, err)
		attempt.ErrorMessage = err.Error()
	}

	// 更新尝试记录
	attempt.Status = core.AttemptStatusSent
	attempt.ResponseCode = responseCode
	attempt.ResponseBody = responseBody
	attempt.UpdatedAt = time.Now()

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
		w.logger.Info("Notification sent successfully for task %s", task.TaskID)
		return
	}

	// 发送失败，处理重试逻辑
	if task.AttemptCount < w.config.MaxRetries {
		// 计算下次重试时间（指数退避）
		nextAttemptAt := calculateNextAttempt(task.AttemptCount, w.config.RetryBackoff)
		// 更新任务状态为待重试
		if err := w.store.UpdateTaskRetry(ctx, task.TaskID, task.AttemptCount+1, nextAttemptAt); err != nil {
			w.logger.Error("Failed to update task retry for task %s: %v", task.TaskID, err)
			return
		}
		w.logger.Info("Notification failed for task %s, will retry at %s", task.TaskID, nextAttemptAt.Format(time.RFC3339))
	} else {
		// 达到最大重试次数，更新任务状态为失败
		if err := w.store.UpdateTaskStatus(ctx, task.TaskID, core.TaskStatusFailed, time.Now()); err != nil {
			w.logger.Error("Failed to update task status to failed for task %s: %v", task.TaskID, err)
			return
		}
		w.logger.Info("Notification failed for task %s after %d attempts, marked as failed", task.TaskID, w.config.MaxRetries)
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

	// 创建HTTP请求
	resp, err := w.httpClient.Do(ctx, task.HTTPMethod, task.TargetURL, headers, []byte(task.Body))
	if err != nil {
		return false, 0, "", err
	}

	// 根据响应码判断是否成功
	// 通常2xx表示成功
	success := resp.StatusCode >= 200 && resp.StatusCode < 300
	return success, resp.StatusCode, string(resp.Body), nil
}

// calculateNextAttempt 计算下次重试时间
// 使用指数退避策略

func calculateNextAttempt(attemptCount int, baseBackoff time.Duration) time.Time {
	// 指数退避: baseBackoff * (2^attemptCount)
	backoff := baseBackoff
	for i := 0; i < attemptCount; i++ {
		backoff *= 2
	}
	return time.Now().Add(backoff)
}