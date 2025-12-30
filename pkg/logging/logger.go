package logging

import (
	"fmt"
	"log"
	"os"
	"strings"
)

// Level 日志级别
type Level int

const (
	// LevelDebug 调试级别
	LevelDebug Level = iota
	// LevelInfo 信息级别
	LevelInfo
	// LevelWarn 警告级别
	LevelWarn
	// LevelError 错误级别
	LevelError
)

// Logger 日志记录器
type Logger struct {
	level  Level
	debug  *log.Logger
	info   *log.Logger
	warn   *log.Logger
	error  *log.Logger
}

// New 创建一个新的日志记录器
func New(levelStr string) *Logger {
	var level Level
	switch strings.ToLower(levelStr) {
	case "debug":
		level = LevelDebug
	case "info":
		level = LevelInfo
	case "warn":
		level = LevelWarn
	case "error":
		level = LevelError
	default:
		level = LevelInfo
	}

	flags := log.Ldate | log.Ltime | log.Lmicroseconds

	return &Logger{
		level:  level,
		debug:  log.New(os.Stdout, "[DEBUG] ", flags),
		info:   log.New(os.Stdout, "[INFO] ", flags),
		warn:   log.New(os.Stdout, "[WARN] ", flags),
		error:  log.New(os.Stderr, "[ERROR] ", flags),
	}
}

// Debug 记录调试日志
func (l *Logger) Debug(format string, v ...interface{}) {
	if l.level <= LevelDebug {
		l.debug.Output(2, fmt.Sprintf(format, v...))
	}
}

// Info 记录信息日志
func (l *Logger) Info(format string, v ...interface{}) {
	if l.level <= LevelInfo {
		l.info.Output(2, fmt.Sprintf(format, v...))
	}
}

// Warn 记录警告日志
func (l *Logger) Warn(format string, v ...interface{}) {
	if l.level <= LevelWarn {
		l.warn.Output(2, fmt.Sprintf(format, v...))
	}
}

// Error 记录错误日志
func (l *Logger) Error(format string, v ...interface{}) {
	if l.level <= LevelError {
		l.error.Output(2, fmt.Sprintf(format, v...))
	}
}

// WithFields 添加字段（简化版）
func (l *Logger) WithFields(fields map[string]interface{}) *Logger {
	// 简化实现，实际项目中可以使用更复杂的结构化日志
	return l
}