package httpclient

import (
	"bytes"
	"context"
	"io/ioutil"
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
	return &Client{
		client: &http.Client{
			Timeout: 30 * time.Second, // 默认超时30秒
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
		req.Header.Set(k, v)
	}

	// 如果没有设置Content-Type，默认设置为application/json
	if req.Header.Get("Content-Type") == "" && len(body) > 0 {
		req.Header.Set("Content-Type", "application/json")
	}

	// 发送请求
	resp, err := c.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	// 读取响应体
	respBody, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	latency := time.Since(startTime)

	// 记录请求信息
	c.logger.Debug("HTTP Request: %s %s, StatusCode: %d, Latency: %v", method, url, resp.StatusCode, latency)

	return &Response{
		StatusCode: resp.StatusCode,
		Body:       respBody,
		Latency:    latency,
	}, nil
}

// Get 发送GET请求
func (c *Client) Get(ctx context.Context, url string, headers map[string]string) (*Response, error) {
	return c.Do(ctx, http.MethodGet, url, headers, nil)
}

// Post 发送POST请求
func (c *Client) Post(ctx context.Context, url string, headers map[string]string, body []byte) (*Response, error) {
	return c.Do(ctx, http.MethodPost, url, headers, body)
}