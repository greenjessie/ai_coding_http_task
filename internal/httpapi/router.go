package httpapi

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"api-notify/internal/core"
	"api-notify/internal/store"
	"api-notify/pkg/logging"
)

// Router HTTP路由器
type Router struct {
	mux    *http.ServeMux
	store  *store.Store
	logger *logging.Logger
}

// NewRouter 创建一个新的路由器
func NewRouter(store *store.Store, logger *logging.Logger) *Router {
	router := &Router{
		mux:    http.NewServeMux(),
		store:  store,
		logger: logger,
	}

	// 注册路由
	router.registerRoutes()

	return router
}

// ServeHTTP 实现http.Handler接口
func (r *Router) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	r.mux.ServeHTTP(w, req)
}

// registerRoutes 注册路由
func (r *Router) registerRoutes() {
	// 创建通知
	r.mux.HandleFunc("/v1/notify", r.handleCreateNotification)
	// 获取通知状态
	r.mux.HandleFunc("/v1/notify/", r.handleNotification)
}

// handleCreateNotification 处理创建通知请求
func (r *Router) handleCreateNotification(w http.ResponseWriter, req *http.Request) {
	if req.Method != http.MethodPost {
		r.writeError(w, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}

	// 解析请求体
	var reqBody CreateNotificationRequest
	if err := json.NewDecoder(req.Body).Decode(&reqBody); err != nil {
		r.writeError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	// 验证请求参数
	if reqBody.PartnerID == "" || reqBody.TargetURL == "" {
		r.writeError(w, http.StatusBadRequest, "PartnerID and TargetURL are required")
		return
	}

	// 检查目标URL是否在白名单域名内
	if !r.isURLInWhitelist(reqBody.TargetURL) {
		r.writeError(w, http.StatusForbidden, "Target URL is not in whitelist")
		return
	}

	// 处理幂等性键
	idempotencyKey := reqBody.IdempotencyKey
	if idempotencyKey == "" {
		idempotencyKey = req.Header.Get("Idempotency-Key")
	}

	// 幂等性校验
	if idempotencyKey != "" {
		// 检查是否已存在相同的幂等键和partner_id的任务
		existingTask, err := r.store.GetTaskByIdempotencyKey(req.Context(), idempotencyKey, reqBody.PartnerID)
		if err != nil {
			r.logger.Error("Failed to check idempotency: %v", err)
			r.writeError(w, http.StatusInternalServerError, "Failed to create notification")
			return
		}
		if existingTask != nil {
			// 返回已存在的任务ID
			r.writeJSON(w, http.StatusOK, CreateNotificationResponse{
				TaskID: existingTask.TaskID,
				Status: string(existingTask.Status),
			})
			return
		}
	}

	// 设置默认值
	httpMethod := reqBody.Method
	if httpMethod == "" {
		httpMethod = "POST"
	}

	maxAttempts := 3 // 默认最大尝试次数

	// 生成任务ID
	taskID := fmt.Sprintf("task_%d_%s", time.Now().UnixNano(), r.generateRandomString(8))

	// 创建任务
	task := &core.NotificationTask{
		TaskID:             taskID,
		PartnerID:          reqBody.PartnerID,
		TargetURL:          reqBody.TargetURL,
		HTTPMethod:         httpMethod,
		Headers:            r.encodeHeaders(reqBody.Headers),
		Body:               string(reqBody.Body),
		IdempotencyKey:     idempotencyKey,
		Priority:           reqBody.Priority,
		Status:             core.TaskStatusPending,
		NextAttemptAt:      time.Now(),
		MaxAttempts:        maxAttempts,
		AttemptCount:       0,
		SuccessCondition:   reqBody.SuccessCondition,
	}

	// 保存任务到数据库
	if err := r.store.CreateTask(req.Context(), task); err != nil {
		r.logger.Error("Failed to create task: %v", err)
		r.writeError(w, http.StatusInternalServerError, "Failed to create notification")
		return
	}

	// 返回响应
	r.writeJSON(w, http.StatusCreated, CreateNotificationResponse{
		TaskID: taskID,
		Status: string(task.Status),
	})
}

// handleNotification 处理获取和取消通知请求
func (r *Router) handleNotification(w http.ResponseWriter, req *http.Request) {
	// 解析任务ID
	taskID, action := r.extractTaskIDAndAction(req.URL.Path)
	if taskID == "" {
		r.writeError(w, http.StatusBadRequest, "Invalid task ID")
		return
	}

	// 根据请求方法和action执行不同操作
	switch {
	case req.Method == http.MethodGet && action == "":
		// 获取通知状态
		r.handleGetNotification(w, req, taskID)
	case req.Method == http.MethodPost && action == "cancel":
		// 取消通知
		r.handleCancelNotification(w, req, taskID)
	default:
		r.writeError(w, http.StatusMethodNotAllowed, "Method not allowed")
	}
}

// handleGetNotification 处理获取通知状态请求
func (r *Router) handleGetNotification(w http.ResponseWriter, req *http.Request, taskID string) {
	// 查询任务
	task, err := r.store.GetTaskByTaskID(req.Context(), taskID)
	if err != nil {
		r.logger.Error("Failed to get task: %v", err)
		r.writeError(w, http.StatusInternalServerError, "Failed to get notification")
		return
	}

	if task == nil {
		r.writeError(w, http.StatusNotFound, "Notification not found")
		return
	}

	// 获取所有尝试记录
	attempts, err := r.store.GetAttemptsByTaskID(req.Context(), taskID)
	if err != nil {
		r.logger.Error("Failed to get attempts: %v", err)
		r.writeError(w, http.StatusInternalServerError, "Failed to get notification")
		return
	}

	// 准备响应
	resp := GetNotificationResponse{
		TaskID:         task.TaskID,
		PartnerID:      task.PartnerID,
		TargetURL:      task.TargetURL,
		Method:         task.HTTPMethod,
		Status:         string(task.Status),
		MaxAttempts:    task.MaxAttempts,
		AttemptCount:   len(attempts),
		CreatedAt:      task.CreatedAt.Format(time.RFC3339),
		UpdatedAt:      task.UpdatedAt.Format(time.RFC3339),
	}

	// 设置下次尝试时间（仅当任务处于非终态时）
	if task.Status == core.TaskStatusPending || task.Status == core.TaskStatusRunning {
		resp.NextAttemptAt = task.NextAttemptAt.Format(time.RFC3339)
	}

	// 设置最近一次尝试摘要（如果有尝试记录）
	if len(attempts) > 0 {
		// 获取最后一次尝试记录
		lastAttempt := attempts[len(attempts)-1]
		
		resp.LastAttemptSummary = &LastAttemptSummary{
			AttemptNo:      lastAttempt.AttemptNo,
			HTTPStatusCode: lastAttempt.HTTPStatusCode,
			ErrorCode:      lastAttempt.ErrorCode,
			ErrorMessage:   lastAttempt.ErrorMessage,
			LatencyMs:      lastAttempt.LatencyMs,
			CreatedAt:      lastAttempt.CreatedAt.Format(time.RFC3339),
		}
	}

	// 返回响应
	r.writeJSON(w, http.StatusOK, resp)
}

// handleCancelNotification 处理取消通知请求
func (r *Router) handleCancelNotification(w http.ResponseWriter, req *http.Request, taskID string) {
	// 查询任务
	task, err := r.store.GetTaskByTaskID(req.Context(), taskID)
	if err != nil {
		r.logger.Error("Failed to get task: %v", err)
		r.writeError(w, http.StatusInternalServerError, "Failed to cancel notification")
		return
	}

	if task == nil {
		r.writeError(w, http.StatusNotFound, "Notification not found")
		return
	}

	// 检查任务是否处于非终态
	if task.Status != core.TaskStatusPending && task.Status != core.TaskStatusRunning {
		r.writeError(w, http.StatusBadRequest, "Cannot cancel a task in terminal state")
		return
	}

	// 更新任务状态为dead
	if err := r.store.UpdateTaskStatus(req.Context(), taskID, core.TaskStatusDead, time.Now()); err != nil {
		r.logger.Error("Failed to cancel task: %v", err)
		r.writeError(w, http.StatusInternalServerError, "Failed to cancel notification")
		return
	}

	// 返回响应
	r.writeJSON(w, http.StatusOK, CancelNotificationResponse{
		TaskID: taskID,
		Status: string(core.TaskStatusDead),
	})
}

// extractTaskIDAndAction 从URL路径中提取任务ID和操作
func (r *Router) extractTaskIDAndAction(path string) (string, string) {
	parts := splitPath(path)
	if len(parts) >= 3 {
		taskID := parts[2]
		if len(parts) >= 4 {
			return taskID, parts[3]
		}
		return taskID, ""
	}
	return "", ""
}

// writeJSON 写入JSON响应
func (r *Router) writeJSON(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(data); err != nil {
		r.logger.Error("Failed to write JSON response: %v", err)
	}
}

// writeError 写入错误响应
func (r *Router) writeError(w http.ResponseWriter, status int, message string) {
	r.writeJSON(w, status, ErrorResponse{
		Code:    status,
		Message: message,
	})
}

// encodeHeaders 将headers编码为JSON字符串
func (r *Router) encodeHeaders(headers map[string]string) string {
	if headers == nil {
		return ""
	}

	data, err := json.Marshal(headers)
	if err != nil {
		r.logger.Error("Failed to encode headers: %v", err)
		return ""
	}

	return string(data)
}

// extractTaskID 从URL路径中提取任务ID
func (r *Router) extractTaskID(path string) string {
	// 简单实现，实际应该使用更健壮的解析方法
	parts := splitPath(path)
	if len(parts) >= 4 {
		return parts[3]
	}
	return ""
}

// splitPath 分割URL路径
func splitPath(path string) []string {
	var parts []string
	part := ""
	for i := 1; i < len(path); i++ {
		if path[i] == '/' {
			if part != "" {
				parts = append(parts, part)
				part = ""
			}
		} else {
			part += string(path[i])
		}
	}
	if part != "" {
		parts = append(parts, part)
	}
	return parts
}

// generateRandomString 生成随机字符串
func (r *Router) generateRandomString(length int) string {
	const charset = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	result := make([]byte, length)
	for i := range result {
		result[i] = charset[int(time.Now().UnixNano())%len(charset)]
	}
	return string(result)
}

// isURLInWhitelist 检查目标URL是否在白名单域名内
// 注意：当前为预留接口，实际实现应从配置中读取白名单
func (r *Router) isURLInWhitelist(targetURL string) bool {
	// 简化实现：暂时允许所有URL
	// 实际实现应该：
	// 1. 解析URL获取域名
	// 2. 检查域名是否在配置的白名单中
	return true
}