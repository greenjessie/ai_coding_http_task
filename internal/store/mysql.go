package store

import (
	"database/sql"
	"fmt"

	_ "github.com/go-sql-driver/mysql"

	"api-notify/internal/config"
	"api-notify/pkg/logging"
)

// Store 数据库存储接口
type Store struct {
	db     *sql.DB
	logger *logging.Logger
}

// New 创建一个新的数据库存储实例
func New(cfg *config.Config, logger *logging.Logger) (*Store, error) {
	// 连接数据库
	db, err := sql.Open("mysql", cfg.Database.DSN)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	// 配置连接池
	db.SetMaxIdleConns(cfg.Database.MaxIdleConns)
	db.SetMaxOpenConns(cfg.Database.MaxOpenConns)
	db.SetConnMaxLifetime(cfg.Database.ConnMaxLifetime)

	// 测试连接
	if err := db.Ping(); err != nil {
		return nil, fmt.Errorf("failed to ping database: %w", err)
	}

	logger.Info("Database connected successfully")

	// 初始化表结构
	if err := initTables(db, logger); err != nil {
		return nil, fmt.Errorf("failed to init tables: %w", err)
	}

	return &Store{
		db:     db,
		logger: logger,
	}, nil
}

// Close 关闭数据库连接
func (s *Store) Close() error {
	return s.db.Close()
}

// DB 获取数据库连接
func (s *Store) DB() *sql.DB {
	return s.db
}

// initTables 初始化表结构
func initTables(db *sql.DB, logger *logging.Logger) error {
	// 创建通知任务表
	taskTableSQL := `
	CREATE TABLE IF NOT EXISTS notification_tasks (
		id BIGINT UNSIGNED AUTO_INCREMENT PRIMARY KEY,
		task_id VARCHAR(64) NOT NULL UNIQUE,
		partner_id VARCHAR(32) NOT NULL,
		target_url VARCHAR(512) NOT NULL,
		http_method VARCHAR(10) NOT NULL DEFAULT 'POST',
		headers TEXT,
		body LONGTEXT,
		idempotency_key VARCHAR(64),
		priority INT NOT NULL DEFAULT 0,
		status VARCHAR(16) NOT NULL DEFAULT 'pending',
		next_attempt_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
		max_attempts INT NOT NULL DEFAULT 3,
		success_condition VARCHAR(256),
		created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
		updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
		INDEX idx_partner_id (partner_id),
		INDEX idx_status (status),
		INDEX idx_next_attempt_at (next_attempt_at)
	) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;
	`

	if _, err := db.Exec(taskTableSQL); err != nil {
		return fmt.Errorf("failed to create notification_tasks table: %w", err)
	}

	// 创建通知尝试记录表
	attemptTableSQL := `
	CREATE TABLE IF NOT EXISTS notification_attempts (
		id BIGINT UNSIGNED AUTO_INCREMENT PRIMARY KEY,
		task_id VARCHAR(64) NOT NULL,
		attempt_number INT NOT NULL,
		status_code INT NOT NULL DEFAULT 0,
		latency_ms BIGINT NOT NULL DEFAULT 0,
		error_message TEXT,
		created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
		INDEX idx_task_id (task_id),
		INDEX idx_created_at (created_at)
	) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;
	`

	if _, err := db.Exec(attemptTableSQL); err != nil {
		return fmt.Errorf("failed to create notification_attempts table: %w", err)
	}

	logger.Info("Database tables initialized successfully")
	return nil
}