package metrics

import (
	"time"

	"api-notify/pkg/logging"
)

// Metrics 指标收集器接口
// 预留接口支持多种监控系统集成（如Prometheus）
type Metrics interface {
	// IncrInboundRequest 增加入站请求计数
	IncrInboundRequest(partnerID string)

	// IncrNotificationSent 增加通知发送计数
	IncrNotificationSent(taskID string, partnerID string, status string, httpStatusCode int)

	// RecordNotificationLatency 记录通知发送延迟
	RecordNotificationLatency(taskID string, partnerID string, latency time.Duration)

	// RecordRetryAttempt 记录重试尝试
	RecordRetryAttempt(taskID string, partnerID string, attemptNo int)

	// IncrDeadTask 增加dead任务计数
	IncrDeadTask(taskID string, partnerID string)

	// GetStats 获取当前统计信息
	GetStats() Stats
}

// Stats 指标统计信息
type Stats struct {
	InboundRequests  int64
	NotificationsSent int64
	SuccessCount     int64
	FailureCount     int64
	AverageLatency   time.Duration
	AverageRetries   float64
	DeadTasks        int64
}

// SimpleMetrics 简单的内存指标收集器
// 用于开发和测试环境，生产环境可替换为Prometheus等实现
type SimpleMetrics struct {
	logger            *logging.Logger
	inboundRequests   int64
	notificationsSent int64
	successCount      int64
	failureCount      int64
	totalLatency      time.Duration
	totalRetries      int64
	retryCount        int64
	deadTasks         int64
}

// NewSimpleMetrics 创建一个新的简单指标收集器
func NewSimpleMetrics(logger *logging.Logger) *SimpleMetrics {
	return &SimpleMetrics{
		logger: logger,
	}
}

// IncrInboundRequest 增加入站请求计数
func (m *SimpleMetrics) IncrInboundRequest(partnerID string) {
	m.inboundRequests++
	m.logger.Debug("Inbound request incremented for partner %s", partnerID)
}

// IncrNotificationSent 增加通知发送计数
func (m *SimpleMetrics) IncrNotificationSent(taskID string, partnerID string, status string, httpStatusCode int) {
	m.notificationsSent++
	if httpStatusCode >= 200 && httpStatusCode < 400 {
		m.successCount++
	} else {
		m.failureCount++
	}
	m.logger.Debug("Notification sent for task %s, partner %s, status %s, http status %d", 
		taskID, partnerID, status, httpStatusCode)
}

// RecordNotificationLatency 记录通知发送延迟
func (m *SimpleMetrics) RecordNotificationLatency(taskID string, partnerID string, latency time.Duration) {
	m.totalLatency += latency
	m.logger.Debug("Notification latency recorded for task %s, partner %s: %v", 
		taskID, partnerID, latency)
}

// RecordRetryAttempt 记录重试尝试
func (m *SimpleMetrics) RecordRetryAttempt(taskID string, partnerID string, attemptNo int) {
	m.totalRetries += int64(attemptNo)
	m.retryCount++
	m.logger.Debug("Retry attempt recorded for task %s, partner %s, attempt %d", 
		taskID, partnerID, attemptNo)
}

// IncrDeadTask 增加dead任务计数
func (m *SimpleMetrics) IncrDeadTask(taskID string, partnerID string) {
	m.deadTasks++
	m.logger.Debug("Dead task incremented for task %s, partner %s", taskID, partnerID)
}

// GetStats 获取当前统计信息
func (m *SimpleMetrics) GetStats() Stats {
	averageLatency := time.Duration(0)
	if m.notificationsSent > 0 {
		averageLatency = m.totalLatency / time.Duration(m.notificationsSent)
	}

	averageRetries := 0.0
	if m.retryCount > 0 {
		averageRetries = float64(m.totalRetries) / float64(m.retryCount)
	}

	return Stats{
		InboundRequests:   m.inboundRequests,
		NotificationsSent: m.notificationsSent,
		SuccessCount:      m.successCount,
		FailureCount:      m.failureCount,
		AverageLatency:    averageLatency,
		AverageRetries:    averageRetries,
		DeadTasks:         m.deadTasks,
	}
}