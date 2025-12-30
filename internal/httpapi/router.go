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

	// 设置默认值
	httpMethod := reqBody.HTTPMethod
	if httpMethod == "" {
		httpMethod = "POST"
	}

	maxAttempts := reqBody.MaxAttempts
	if maxAttempts == 0 {
		maxAttempts = 3
	}

	// 生成任务ID
	taskID := fmt.Sprintf("task_%d_%s", time.Now().UnixNano(), r.generateRandomString(8))

	// 创建任务
	task := &core.NotificationTask{
		TaskID:         taskID,
		PartnerID:      reqBody.PartnerID,
		TargetURL:      reqBody.TargetURL,
		HTTPMethod:     httpMethod,
		Headers:        r.encodeHeaders(reqBody.Headers),
		Body:           reqBody.Body,
		IdempotencyKey: reqBody.IdempotencyKey,
		Priority:       reqBody.Priority,
		Status:         core.TaskStatusPending,
		NextAttemptAt:  time.Now(),
		MaxAttempts:    maxAttempts,
		SuccessCondition: reqBody.SuccessCondition,
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

	// 获取尝试次数
	attemptCount, err := r.store.GetAttemptCount(req.Context(), taskID)
	if err != nil {
		r.logger.Error("Failed to get attempt count: %v", err)
		r.writeError(w, http.StatusInternalServerError, "Failed to get notification")
		return
	}

	// 返回响应
	r.writeJSON(w, http.StatusOK, GetNotificationResponse{
		TaskID:         task.TaskID,
		PartnerID:      task.PartnerID,
		TargetURL:      task.TargetURL,
		HTTPMethod:     task.HTTPMethod,
		Status:         string(task.Status),
		NextAttemptAt:  task.NextAttemptAt.Format(time.RFC3339),
		MaxAttempts:    task.MaxAttempts,
		AttemptCount:   attemptCount,
		CreatedAt:      task.CreatedAt.Format(time.RFC3339),
		UpdatedAt:      task.UpdatedAt.Format(time.RFC3339),
	})
}

// handleCancelNotification 处理取消通知请求
func (r *Router) handleCancelNotification(w http.ResponseWriter, req *http.Request, taskID string) {
	// 更新任务状态为已取消
	if err := r.store.UpdateTaskStatus(req.Context(), taskID, core.TaskStatusCancelled, time.Now()); err != nil {
		r.logger.Error("Failed to cancel task: %v", err)
		r.writeError(w, http.StatusInternalServerError, "Failed to cancel notification")
		return
	}

	// 返回成功响应
	w.WriteHeader(http.StatusOK)
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