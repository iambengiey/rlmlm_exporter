package level

import (
	"strings"

	"github.com/go-kit/log"
)

type loggerWithLevel interface {
	LogWithLevel(level string, keyvals ...interface{}) error
}

type levelLogger struct {
	base  log.Logger
	level string
}

func (l levelLogger) Log(keyvals ...interface{}) error {
	if lw, ok := l.base.(loggerWithLevel); ok {
		return lw.LogWithLevel(l.level, keyvals...)
	}
	// Fallback to embedding level key/value.
	keyvals = append([]interface{}{"level", l.level}, keyvals...)
	return l.base.Log(keyvals...)
}

// Debug returns a Logger that emits messages at debug level.
func Debug(logger log.Logger) log.Logger {
	return levelLogger{base: logger, level: "debug"}
}

// Info returns a Logger that emits messages at info level.
func Info(logger log.Logger) log.Logger {
	return levelLogger{base: logger, level: "info"}
}

// Warn returns a Logger that emits messages at warn level.
func Warn(logger log.Logger) log.Logger {
	return levelLogger{base: logger, level: "warn"}
}

// Error returns a Logger that emits messages at error level.
func Error(logger log.Logger) log.Logger {
	return levelLogger{base: logger, level: "error"}
}

// ParseLevel converts a textual level into the internal representation understood by the std logger.
func ParseLevel(level string) log.Level {
	switch strings.ToLower(level) {
	case "debug":
		return log.LevelDebug
	case "info":
		return log.LevelInfo
	case "warn", "warning":
		return log.LevelWarn
	case "error":
		return log.LevelError
	default:
		return log.LevelInfo
	}
}
