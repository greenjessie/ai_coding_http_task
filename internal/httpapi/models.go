package httpapi

// CreateNotificationRequest 创建通知请求
type CreateNotificationRequest struct {
	PartnerID        string            `json:"partner_id" validate:"required"`
	TargetURL        string            `json:"target_url" validate:"required,url"`
	HTTPMethod       string            `json:"http_method" validate:"omitempty,oneof=GET POST PUT DELETE"`
	Headers          map[string]string `json:"headers"`
	Body             string            `json:"body"`
	IdempotencyKey   string            `json:"idempotency_key"`
	Priority         int               `json:"priority"`
	MaxAttempts      int               `json:"max_attempts" validate:"omitempty,min=1,max=10"`
	SuccessCondition string            `json:"success_condition"`
}

// CreateNotificationResponse 创建通知响应
type CreateNotificationResponse struct {
	TaskID string `json:"task_id"`
}

// GetNotificationResponse 获取通知响应
type GetNotificationResponse struct {
	TaskID         string `json:"task_id"`
	PartnerID      string `json:"partner_id"`
	TargetURL      string `json:"target_url"`
	HTTPMethod     string `json:"http_method"`
	Status         string `json:"status"`
	NextAttemptAt  string `json:"next_attempt_at"`
	MaxAttempts    int    `json:"max_attempts"`
	AttemptCount   int    `json:"attempt_count"`
	CreatedAt      string `json:"created_at"`
	UpdatedAt      string `json:"updated_at"`
}

// ErrorResponse 错误响应
type ErrorResponse struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}