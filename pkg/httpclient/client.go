package httpclient

import (
	"bytes"
	"context"
	"io/ioutil"
	"net"
	"net/http"
	"time"

	"api-notify/pkg/logging"
)

// Client HTTP客户端
type Client struct {
	client  *http.Client
	logger  *logging.Logger
}

// Response HTTP响应
type Response struct {
	StatusCode int
	Body       []byte
	Latency    time.Duration
}

// New 创建一个新的HTTP客户端
func New(logger *logging.Logger) *Client {
	// 配置传输层
	transport := &http.Transport{
		// 限制最大连接数
		MaxIdleConns:        100,
		MaxIdleConnsPerHost: 10,
		// 连接超时和读取超时
		DialContext: (&net.Dialer{
			Timeout:   5 * time.Second,  // 连接超时
			KeepAlive: 30 * time.Second,
		}).DialContext,
		TLSHandshakeTimeout:   5 * time.Second,
		ResponseHeaderTimeout: 10 * time.Second,
	}

	return &Client{
		client: &http.Client{
			Timeout:   10 * time.Second, // 总超时时间（3～10s）
			Transport: transport,
		}, 
		logger: logger,
	}
}

// Do 发送HTTP请求
func (c *Client) Do(ctx context.Context, method, url string, headers map[string]string, body []byte) (*Response, error) {
	startTime := time.Now()

	// 创建请求
	req, err := http.NewRequestWithContext(ctx, method, url, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}

	// 设置请求头
	for k, v := range headers {
		if isSensitiveHeader(k) {
			req.Header.Set(k, "[REDACTED]")
		} else {
			req.Header.Set(k, v)
		}
	}

	// 如果没有设置Content-Type，默认设置为application/json
	if req.Header.Get("Content-Type") == "" && len(body) > 0 {
		req.Header.Set("Content-Type", "application/json")
	}

	// 发送请求
	resp, err := c.client.Do(req)
	if err != nil {
		// 记录错误日志
		c.logger.Error("HTTP Request failed: %s %s, Error: %v", method, sanitizeURL(url), err)
		return nil, err
	}
	defer resp.Body.Close()

	// 读取响应体
	respBody, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		c.logger.Error("Failed to read response body: %v", err)
		return nil, err
	}

	latency := time.Since(startTime)

	// 截断响应体日志
	respBodyLog := string(respBody)
	if len(respBodyLog) > 100 {
		respBodyLog = respBodyLog[:100] + "..."
	}

	// 记录请求信息（脱敏）
	c.logger.Debug("HTTP Request: %s %s, StatusCode: %d, Latency: %v, ResponseBody: %s", 
		method, sanitizeURL(url), resp.StatusCode, latency, respBodyLog)

	return &Response{
		StatusCode: resp.StatusCode,
		Body:       respBody,
		Latency:    latency,
	}, nil
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

// sanitizeURL 脱敏URL
func sanitizeURL(url string) string {
	// 简单脱敏示例，实际可以根据需要扩展
	return url
}

// Get 发送GET请求
func (c *Client) Get(ctx context.Context, url string, headers map[string]string) (*Response, error) {
	return c.Do(ctx, http.MethodGet, url, headers, nil)
}

// Post 发送POST请求
func (c *Client) Post(ctx context.Context, url string, headers map[string]string, body []byte) (*Response, error) {
	return c.Do(ctx, http.MethodPost, url, headers, body)
}