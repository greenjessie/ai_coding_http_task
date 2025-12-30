package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"api-notify/internal/config"
	"api-notify/internal/dispatcher"
	"api-notify/internal/httpapi"
	"api-notify/internal/store"
	"api-notify/pkg/httpclient"
	"api-notify/pkg/logging"
)

func main() {
	// 1. 初始化日志
	logger := logging.New("info")
	logger.Info("Starting API notification service...")

	// 2. 加载配置
	cfg, err := config.Load()
	if err != nil {
		logger.Error("Failed to load configuration: %v", err)
		log.Fatalf("Failed to load configuration: %v", err)
	}
	logger.Info("Configuration loaded successfully")

	// 3. 初始化数据库
	store, err := store.New(cfg, logger)
	if err != nil {
		logger.Error("Failed to initialize database: %v", err)
		log.Fatalf("Failed to initialize database: %v", err)
	}
	defer store.Close()
	logger.Info("Database initialized successfully")

	// 4. 初始化HTTP客户端
	httpClient := httpclient.New(logger)
	logger.Info("HTTP client initialized successfully")

	// 5. 创建HTTP路由
	router := httpapi.NewRouter(store, logger)
	logger.Info("HTTP router initialized successfully")

	// 6. 创建Worker
	workerConfig := dispatcher.WorkerConfig{
		Interval:          cfg.Worker.PollInterval,
		BatchSize:         100, // 默认批量大小
		MaxRetries:        cfg.Worker.MaxAttempts,
		RetryBackoff:      5 * time.Second, // 默认重试间隔
		ConcurrentWorkers: cfg.Worker.Concurrency,
	}

	worker := dispatcher.NewWorker(logger, store, httpClient, workerConfig)

	// 7. 创建HTTP服务器
	server := &http.Server{
		Addr:    fmt.Sprintf(":%d", cfg.Server.Port),
		Handler: router,
	}

	// 8. 创建上下文用于优雅退出
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	// 9. 启动Worker
	worker.Start(ctx)

	// 10. 启动HTTP服务器
	go func() {
		logger.Info("HTTP server starting on port %d", cfg.Server.Port)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Error("Failed to start HTTP server: %v", err)
			os.Exit(1)
		}
	}()

	// 11. 等待信号
	<-ctx.Done()
	logger.Info("Shutting down server...")

	// 12. 停止Worker
	worker.Stop()

	// 13. 关闭HTTP服务器
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := server.Shutdown(shutdownCtx); err != nil {
		logger.Error("HTTP server forced to shutdown: %v", err)
	} else {
		logger.Info("HTTP server shut down gracefully")
	}

	logger.Info("API notification service stopped successfully")
}