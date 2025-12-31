package httpapi

import (
	"encoding/json"
)

// CreateNotificationRequest 创建通知请求
type CreateNotificationRequest struct {
	TargetURL      string                 `json:"target_url" validate:"required"`
	Method         string                 `json:"method" validate:"omitempty,oneof=GET POST PUT DELETE"`
	Headers        map[string]string      `json:"headers"`
	Body           json.RawMessage        `json:"body"`
	IdempotencyKey string                 `json:"idempotency_key"`
	PartnerID      string                 `json:"partner_id" validate:"required"`
	Priority       int                    `json:"priority"`
	SuccessCondition string               `json:"success_condition"`
}

// CreateNotificationResponse 创建通知响应
type CreateNotificationResponse struct {
	TaskID string `json:"task_id"`
	Status string `json:"status"`
}

// GetNotificationResponse 获取通知响应
type GetNotificationResponse struct {
	TaskID             string                    `json:"task_id"`
	PartnerID          string                    `json:"partner_id"`
	TargetURL          string                    `json:"target_url"`
	Method             string                    `json:"method"`
	Status             string                    `json:"status"`
	NextAttemptAt      string                    `json:"next_attempt_at,omitempty"`
	MaxAttempts        int                       `json:"max_attempts"`
	AttemptCount       int                       `json:"attempt_count"`
	LastAttemptSummary *LastAttemptSummary       `json:"last_attempt_summary,omitempty"`
	CreatedAt          string                    `json:"created_at"`
	UpdatedAt          string                    `json:"updated_at"`
}

// LastAttemptSummary 最近一次尝试摘要
type LastAttemptSummary struct {
	AttemptNo      int    `json:"attempt_no"`
	HTTPStatusCode int    `json:"http_status_code"`
	ErrorCode      string `json:"error_code"`
	ErrorMessage   string `json:"error_message"`
	LatencyMs      int64  `json:"latency_ms"`
	CreatedAt      string `json:"created_at"`
}

// ErrorResponse 错误响应
type ErrorResponse struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

// CancelNotificationResponse 取消通知响应
type CancelNotificationResponse struct {
	TaskID string `json:"task_id"`
	Status string `json:"status"`
}