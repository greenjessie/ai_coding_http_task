package core

import (
	"time"
)

// TaskStatus 通知任务状态
type TaskStatus string

const (
	// TaskStatusPending 等待处理
	TaskStatusPending TaskStatus = "pending"
	// TaskStatusRunning 正在处理
	TaskStatusRunning TaskStatus = "running"
	// TaskStatusSucceeded 处理成功
	TaskStatusSucceeded TaskStatus = "succeeded"
	// TaskStatusFailed 处理失败
	TaskStatusFailed TaskStatus = "failed"
	// TaskStatusCancelled 已取消
	TaskStatusCancelled TaskStatus = "cancelled"
	// TaskStatusDead 任务死亡（超过最大重试次数）
	TaskStatusDead TaskStatus = "dead"
)

// AttemptStatus 通知尝试状态
type AttemptStatus string

const (
	// AttemptStatusPending 等待处理
	AttemptStatusPending AttemptStatus = "pending"
	// AttemptStatusSent 已发送
	AttemptStatusSent AttemptStatus = "sent"
	// AttemptStatusSuccess 发送成功
	AttemptStatusSuccess AttemptStatus = "success"
	// AttemptStatusFailed 发送失败
	AttemptStatusFailed AttemptStatus = "failed"
)

// NotificationAttempt 通知尝试记录
type NotificationAttempt struct {
	ID            uint64        `json:"id"`
	TaskID        string        `json:"task_id"` // 外键，关联 notification_tasks.task_id
	PartnerID     string        `json:"partner_id"`
	Status        AttemptStatus `json:"status"`
	ResponseCode  int           `json:"response_code"`
	ResponseBody  string        `json:"response_body"`
	ErrorMessage  string        `json:"error_message"`
	CreatedAt     time.Time     `json:"created_at"`
	UpdatedAt     time.Time     `json:"updated_at"`
}

// NotificationTask 通知任务实体
type NotificationTask struct {
	ID             uint64        `json:"id"`
	TaskID         string        `json:"task_id"`
	PartnerID      string        `json:"partner_id"`
	TargetURL      string        `json:"target_url"`
	HTTPMethod     string        `json:"http_method"`
	Headers        string        `json:"headers"` // JSON 格式的请求头
	Body           string        `json:"body"` // 请求体
	IdempotencyKey string        `json:"idempotency_key"`
	Priority       int           `json:"priority"`
	Status         TaskStatus    `json:"status"`
	NextAttemptAt  time.Time     `json:"next_attempt_at"`
	MaxAttempts    int           `json:"max_attempts"`
	AttemptCount   int           `json:"attempt_count"` // 当前尝试次数
	SuccessCondition string       `json:"success_condition"` // 自定义成功条件
	CreatedAt      time.Time     `json:"created_at"`
	UpdatedAt      time.Time     `json:"updated_at"`
}