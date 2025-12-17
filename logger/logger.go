package logger

import (
	"fmt"
	"log"
	"os"
	"strings"
	"sync"
)

// LogLevel 日志级别
type LogLevel int

const (
	LevelDebug LogLevel = iota
	LevelInfo
	LevelWarn
	LevelError
)

var (
	currentLevel LogLevel = LevelInfo
	logger       *log.Logger
	mu           sync.RWMutex
	logFile      *os.File
)

// Init 初始化日志系统
func Init(level, filePath string) error {
	mu.Lock()
	defer mu.Unlock()

	// 设置日志级别
	currentLevel = parseLevel(level)

	// 只输出到标准输出
	logger = log.New(os.Stdout, "", log.LstdFlags|log.Lshortfile)

	return nil
}

// Close 关闭日志系统
func Close() {
	mu.Lock()
	defer mu.Unlock()

	if logFile != nil {
		logFile.Close()
		logFile = nil
	}
}

// SetLevel 设置日志级别
func SetLevel(level string) {
	mu.Lock()
	defer mu.Unlock()
	currentLevel = parseLevel(level)
}

// parseLevel 解析日志级别字符串
func parseLevel(level string) LogLevel {
	switch strings.ToLower(level) {
	case "debug":
		return LevelDebug
	case "info":
		return LevelInfo
	case "warn", "warning":
		return LevelWarn
	case "error":
		return LevelError
	default:
		return LevelInfo
	}
}

// shouldLog 判断是否应该记录日志
func shouldLog(level LogLevel) bool {
	mu.RLock()
	defer mu.RUnlock()
	return level >= currentLevel
}

// Debug 调试日志
func Debug(format string, v ...interface{}) {
	if shouldLog(LevelDebug) {
		logWithLevel("DEBUG", format, v...)
	}
}

// Info 信息日志
func Info(format string, v ...interface{}) {
	if shouldLog(LevelInfo) {
		logWithLevel("INFO", format, v...)
	}
}

// Warn 警告日志
func Warn(format string, v ...interface{}) {
	if shouldLog(LevelWarn) {
		logWithLevel("WARN", format, v...)
	}
}

// Error 错误日志
func Error(format string, v ...interface{}) {
	if shouldLog(LevelError) {
		logWithLevel("ERROR", format, v...)
	}
}

// Fatal 致命错误日志（会终止程序）
func Fatal(format string, v ...interface{}) {
	logWithLevel("FATAL", format, v...)
	os.Exit(1)
}

// logWithLevel 带级别的日志输出
func logWithLevel(level, format string, v ...interface{}) {
	mu.RLock()
	l := logger
	mu.RUnlock()

	if l == nil {
		// 如果日志系统未初始化，使用标准log
		log.Printf("[%s] "+format, append([]interface{}{level}, v...)...)
		return
	}

	msg := fmt.Sprintf(format, v...)
	l.Output(3, fmt.Sprintf("[%s] %s", level, msg))
}

// Printf 兼容标准库log.Printf
func Printf(format string, v ...interface{}) {
	Info(format, v...)
}

// Println 兼容标准库log.Println
func Println(v ...interface{}) {
	Info("%s", fmt.Sprintln(v...))
}
