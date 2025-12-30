package config

import (
	"os"
	"strconv"
	"time"
)

// Config 应用配置结构体
type Config struct {
	// Server 服务器配置
	Server struct {
		Port        int           `json:"port"`
		ReadTimeout time.Duration `json:"read_timeout"`
		WriteTimeout time.Duration `json:"write_timeout"`
	}

	// Database 数据库配置
	Database struct {
		DSN          string        `json:"dsn"`
		MaxIdleConns int           `json:"max_idle_conns"`
		MaxOpenConns int           `json:"max_open_conns"`
		ConnMaxLifetime time.Duration `json:"conn_max_lifetime"`
	}

	// Worker Worker配置
	Worker struct {
		Concurrency   int           `json:"concurrency"`
		PollInterval  time.Duration `json:"poll_interval"`
		MaxAttempts   int           `json:"max_attempts"`
	}

	// Security 安全配置
	Security struct {
		AllowedDomains []string     `json:"allowed_domains"`
	}

	// Log 日志配置
	Log struct {
		Level string `json:"level"`
	}
}

// Load 加载配置
func Load() (*Config, error) {
	cfg := &Config{}

	// 默认配置
	cfg.Server.Port = 8080
	cfg.Server.ReadTimeout = 10 * time.Second
	cfg.Server.WriteTimeout = 10 * time.Second

	cfg.Database.DSN = getEnv("DB_DSN", "root:password@tcp(127.0.0.1:3306)/api_notify?charset=utf8mb4&parseTime=True&loc=Local")
	cfg.Database.MaxIdleConns = getEnvAsInt("DB_MAX_IDLE_CONNS", 10)
	cfg.Database.MaxOpenConns = getEnvAsInt("DB_MAX_OPEN_CONNS", 100)
	cfg.Database.ConnMaxLifetime = 30 * time.Minute

	cfg.Worker.Concurrency = getEnvAsInt("WORKER_CONCURRENCY", 5)
	cfg.Worker.PollInterval = time.Duration(getEnvAsInt("WORKER_POLL_INTERVAL", 5)) * time.Second
	cfg.Worker.MaxAttempts = getEnvAsInt("WORKER_MAX_ATTEMPTS", 3)

	cfg.Security.AllowedDomains = []string{"*"} // 默认允许所有域名
	cfg.Log.Level = getEnv("LOG_LEVEL", "info")

	return cfg, nil
}

// getEnv 获取环境变量，如果不存在则返回默认值
func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

// getEnvAsInt 获取环境变量并转换为整数，如果不存在或转换失败则返回默认值
func getEnvAsInt(key string, defaultValue int) int {
	valueStr := getEnv(key, "")
	if value, err := strconv.Atoi(valueStr); err == nil {
		return value
	}
	return defaultValue
}