package store

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"api-notify/internal/core"
)

// CreateTask 创建通知任务
func (s *Store) CreateTask(ctx context.Context, task *core.NotificationTask) error {
	query := `
	INSERT INTO notification_tasks (
		task_id, partner_id, target_url, http_method, headers, body, 
		idempotency_key, priority, status, next_attempt_at, max_attempts, success_condition
	) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`

	_, err := s.db.ExecContext(
		ctx,
		query,
		task.TaskID,
		task.PartnerID,
		task.TargetURL,
		task.HTTPMethod,
		task.Headers,
		task.Body,
		task.IdempotencyKey,
		task.Priority,
		task.Status,
		task.NextAttemptAt,
		task.MaxAttempts,
		task.SuccessCondition,
	)

	if err != nil {
		return fmt.Errorf("failed to create task: %w", err)
	}

	return nil
}

// GetTaskByID 根据ID查询任务
func (s *Store) GetTaskByID(ctx context.Context, id uint64) (*core.NotificationTask, error) {
	query := `
	SELECT 
		id, task_id, partner_id, target_url, http_method, headers, body, 
		idempotency_key, priority, status, next_attempt_at, max_attempts, success_condition,
		created_at, updated_at
	FROM notification_tasks WHERE id = ?
	`

	var task core.NotificationTask
	err := s.db.QueryRowContext(ctx, query, id).Scan(
		&task.ID,
		&task.TaskID,
		&task.PartnerID,
		&task.TargetURL,
		&task.HTTPMethod,
		&task.Headers,
		&task.Body,
		&task.IdempotencyKey,
		&task.Priority,
		&task.Status,
		&task.NextAttemptAt,
		&task.MaxAttempts,
		&task.SuccessCondition,
		&task.CreatedAt,
		&task.UpdatedAt,
	)

	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to get task by id: %w", err)
	}

	return &task, nil
}

// GetTaskByTaskID 根据TaskID查询任务
func (s *Store) GetTaskByTaskID(ctx context.Context, taskID string) (*core.NotificationTask, error) {
	query := `
	SELECT 
		id, task_id, partner_id, target_url, http_method, headers, body, 
		idempotency_key, priority, status, next_attempt_at, max_attempts, success_condition,
		created_at, updated_at
	FROM notification_tasks WHERE task_id = ?
	`

	var task core.NotificationTask
	err := s.db.QueryRowContext(ctx, query, taskID).Scan(
		&task.ID,
		&task.TaskID,
		&task.PartnerID,
		&task.TargetURL,
		&task.HTTPMethod,
		&task.Headers,
		&task.Body,
		&task.IdempotencyKey,
		&task.Priority,
		&task.Status,
		&task.NextAttemptAt,
		&task.MaxAttempts,
		&task.SuccessCondition,
		&task.CreatedAt,
		&task.UpdatedAt,
	)

	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to get task by task_id: %w", err)
	}

	return &task, nil
}

// GetTaskByIdempotencyKey 根据幂等键和partner_id查询任务
func (s *Store) GetTaskByIdempotencyKey(ctx context.Context, idempotencyKey, partnerID string) (*core.NotificationTask, error) {
	query := `
	SELECT 
		id, task_id, partner_id, target_url, http_method, headers, body, 
		idempotency_key, priority, status, next_attempt_at, max_attempts, success_condition,
		created_at, updated_at
	FROM notification_tasks WHERE idempotency_key = ? AND partner_id = ?
	`

	var task core.NotificationTask
	err := s.db.QueryRowContext(ctx, query, idempotencyKey, partnerID).Scan(
		&task.ID,
		&task.TaskID,
		&task.PartnerID,
		&task.TargetURL,
		&task.HTTPMethod,
		&task.Headers,
		&task.Body,
		&task.IdempotencyKey,
		&task.Priority,
		&task.Status,
		&task.NextAttemptAt,
		&task.MaxAttempts,
		&task.SuccessCondition,
		&task.CreatedAt,
		&task.UpdatedAt,
	)

	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to get task by idempotency key: %w", err)
	}

	return &task, nil
}

// GetPendingTasks 获取待处理的任务（带行级锁避免重复消费）
func (s *Store) GetPendingTasks(ctx context.Context, limit int) ([]*core.NotificationTask, error) {
	// 使用MySQL行级锁，将状态更新为running并锁定行
	query := `
	UPDATE notification_tasks 
	SET status = ? 
	WHERE status IN (?, ?, ?) AND next_attempt_at <= NOW()
	ORDER BY priority DESC, next_attempt_at ASC
	LIMIT ?
	`

	// 先将任务状态更新为running
	_, err := s.db.ExecContext(
		ctx,
		query,
		core.TaskStatusRunning,
		core.TaskStatusPending,
		core.TaskStatusFailed,
		core.TaskStatusRunning, // 包含running状态以处理可能的中断恢复
		limit,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to update task status to running: %w", err)
	}

	// 再查询已锁定的任务
	selectQuery := `
	SELECT 
		id, task_id, partner_id, target_url, http_method, headers, body, 
		idempotency_key, priority, status, next_attempt_at, max_attempts, success_condition,
		created_at, updated_at
	FROM notification_tasks 
	WHERE status = ?
	ORDER BY priority DESC, next_attempt_at ASC
	LIMIT ?
	`

	rows, err := s.db.QueryContext(
		ctx,
		selectQuery,
		core.TaskStatusRunning,
		limit,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to get pending tasks: %w", err)
	}
	defer rows.Close()

	tasks := make([]*core.NotificationTask, 0, limit)
	for rows.Next() {
		var task core.NotificationTask
		if err := rows.Scan(
			&task.ID,
			&task.TaskID,
			&task.PartnerID,
			&task.TargetURL,
			&task.HTTPMethod,
			&task.Headers,
			&task.Body,
			&task.IdempotencyKey,
			&task.Priority,
			&task.Status,
			&task.NextAttemptAt,
			&task.MaxAttempts,
			&task.SuccessCondition,
			&task.CreatedAt,
			&task.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("failed to scan task: %w", err)
		}
		tasks = append(tasks, &task)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("rows iteration error: %w", err)
	}

	return tasks, nil
}

// UpdateTaskStatus 更新任务状态
func (s *Store) UpdateTaskStatus(ctx context.Context, taskID string, status core.TaskStatus, nextAttemptAt time.Time) error {
	query := `
	UPDATE notification_tasks 
	SET status = ?, next_attempt_at = ? 
	WHERE task_id = ?
	`

	_, err := s.db.ExecContext(ctx, query, status, nextAttemptAt, taskID)
	if err != nil {
		return fmt.Errorf("failed to update task status: %w", err)
	}

	return nil
}

// RecordAttempt 记录尝试结果
func (s *Store) RecordAttempt(ctx context.Context, attempt *core.NotificationAttempt) error {
	query := `
	INSERT INTO notification_attempts (
		task_id, attempt_no, status, http_status_code, error_code, error_message, latency_ms, created_at
	) VALUES (?, ?, ?, ?, ?, ?, ?, ?)
	`

	_, err := s.db.ExecContext(
		ctx,
		query,
		attempt.TaskID,
		attempt.AttemptNo,
		attempt.Status,
		attempt.HTTPStatusCode,
		attempt.ErrorCode,
		attempt.ErrorMessage,
		attempt.LatencyMs,
		attempt.CreatedAt,
	)

	if err != nil {
		return fmt.Errorf("failed to record attempt: %w", err)
	}

	return nil
}

// GetAttemptCount 获取任务尝试次数
func (s *Store) GetAttemptCount(ctx context.Context, taskID string) (int, error) {
	query := "SELECT COUNT(*) FROM notification_attempts WHERE task_id = ?"

	var count int
	err := s.db.QueryRowContext(ctx, query, taskID).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("failed to get attempt count: %w", err)
	}

	return count, nil
}

// GetAttemptsByTaskID 根据TaskID获取所有尝试记录
func (s *Store) GetAttemptsByTaskID(ctx context.Context, taskID string) ([]*core.NotificationAttempt, error) {
	query := `
	SELECT 
		id, task_id, attempt_no, status, http_status_code, error_code, error_message, latency_ms, created_at
	FROM notification_attempts 
	WHERE task_id = ? 
	ORDER BY created_at ASC
	`

	rows, err := s.db.QueryContext(ctx, query, taskID)
	if err != nil {
		return nil, fmt.Errorf("failed to get attempts by task_id: %w", err)
	}
	defer rows.Close()

	attempts := make([]*core.NotificationAttempt, 0)
	for rows.Next() {
		var attempt core.NotificationAttempt
		if err := rows.Scan(
			&attempt.ID,
			&attempt.TaskID,
			&attempt.AttemptNo,
			&attempt.Status,
			&attempt.HTTPStatusCode,
			&attempt.ErrorCode,
			&attempt.ErrorMessage,
			&attempt.LatencyMs,
			&attempt.CreatedAt,
		); err != nil {
			return nil, fmt.Errorf("failed to scan attempt: %w", err)
		}
		attempts = append(attempts, &attempt)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("rows iteration error: %w", err)
	}

	return attempts, nil
}

// UpdateTaskRetry 更新任务重试信息
func (s *Store) UpdateTaskRetry(ctx context.Context, taskID string, attemptCount int, nextAttemptAt time.Time) error {
	query := `
	UPDATE notification_tasks 
	SET attempt_count = ?, next_attempt_at = ?, updated_at = ? 
	WHERE task_id = ?
	`

	_, err := s.db.ExecContext(ctx, query, attemptCount, nextAttemptAt, time.Now(), taskID)
	if err != nil {
		return fmt.Errorf("failed to update task retry: %w", err)
	}

	return nil
}