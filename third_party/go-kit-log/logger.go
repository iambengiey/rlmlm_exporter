package log

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"runtime"
	"strings"
	"sync"
	"time"
)

// Logger is a minimal structured logger compatible with the go-kit log API.
type Logger interface {
	Log(keyvals ...interface{}) error
}

// Valuer mirrors the go-kit Valuer concept and allows lazy values.
type Valuer func() interface{}

type leveledLogger interface {
	LogWithLevel(level string, keyvals ...interface{}) error
}

type stdLogger struct {
	mu      sync.Mutex
	writer  io.Writer
	level   Level
	context []interface{}
	format  string
}

// Level represents the minimum level that will be emitted.
type Level int

const (
	LevelDebug Level = iota
	LevelInfo
	LevelWarn
	LevelError
)

// NewStdLogger builds a Logger that writes to writer with the provided minimum level.
func NewStdLogger(writer io.Writer, lvl Level, format string) Logger {
	if writer == nil {
		writer = os.Stdout
	}
	if format == "" {
		format = "logfmt"
	}
	return &stdLogger{writer: writer, level: lvl, format: strings.ToLower(format)}
}

// NewNopLogger returns a Logger that discards all output.
func NewNopLogger() Logger {
	return nopLogger{}
}

// With returns a Logger that prepends the supplied keyvals to each log line.
func With(logger Logger, keyvals ...interface{}) Logger {
	switch l := logger.(type) {
	case *stdLogger:
		ctx := append([]interface{}{}, l.context...)
		ctx = append(ctx, keyvals...)
		return &stdLogger{writer: l.writer, level: l.level, context: ctx, format: l.format}
	case contextLogger:
		ctx := append([]interface{}{}, l.ctx...)
		ctx = append(ctx, keyvals...)
		return contextLogger{Logger: l.Logger, ctx: ctx}
	default:
		return contextLogger{Logger: logger, ctx: append([]interface{}{}, keyvals...)}
	}
}

// DefaultTimestampUTC matches the go-kit helper returning the current time when evaluated.
func DefaultTimestampUTC() Valuer {
	return func() interface{} {
		return time.Now().UTC().Format(time.RFC3339Nano)
	}
}

// DefaultCaller mirrors the go-kit helper and returns the calling location when evaluated.
func DefaultCaller() Valuer {
	return func() interface{} {
		if _, file, line, ok := runtime.Caller(2); ok {
			if idx := strings.LastIndex(file, "/"); idx >= 0 {
				file = file[idx+1:]
			}
			return fmt.Sprintf("%s:%d", file, line)
		}
		return "unknown:0"
	}
}

type contextLogger struct {
	Logger
	ctx []interface{}
}

func (c contextLogger) Log(keyvals ...interface{}) error {
	keyvals = append(c.ctx, keyvals...)
	return c.Logger.Log(keyvals...)
}

type nopLogger struct{}

func (nopLogger) Log(keyvals ...interface{}) error { return nil }

func (l *stdLogger) Log(keyvals ...interface{}) error {
	l.mu.Lock()
	defer l.mu.Unlock()

	pairs := append([]interface{}{}, l.context...)
	pairs = append(pairs, keyvals...)
	evaluated := evaluateValuers(pairs)
	_, err := fmt.Fprintln(l.writer, formatKeyvals(l.format, evaluated...))
	return err
}

func (l *stdLogger) LogWithLevel(level string, keyvals ...interface{}) error {
	if !l.shouldLog(level) {
		return nil
	}
	keyvals = append([]interface{}{"level", level}, keyvals...)
	return l.Log(keyvals...)
}

func (l *stdLogger) shouldLog(level string) bool {
	switch strings.ToLower(level) {
	case "debug":
		return l.level <= LevelDebug
	case "info":
		return l.level <= LevelInfo
	case "warn", "warning":
		return l.level <= LevelWarn
	case "error":
		return l.level <= LevelError
	default:
		return true
	}
}

func evaluateValuers(keyvals []interface{}) []interface{} {
	out := make([]interface{}, len(keyvals))
	for i, kv := range keyvals {
		if valuer, ok := kv.(Valuer); ok {
			out[i] = valuer()
			continue
		}
		out[i] = kv
	}
	return out
}

func formatKeyvals(format string, keyvals ...interface{}) string {
	if len(keyvals)%2 != 0 {
		keyvals = append(keyvals, "")
	}
	switch strings.ToLower(format) {
	case "json":
		payload := make(map[string]interface{}, len(keyvals)/2)
		for i := 0; i < len(keyvals); i += 2 {
			payload[fmt.Sprint(keyvals[i])] = keyvals[i+1]
		}
		encoded, err := json.Marshal(payload)
		if err != nil {
			return fmt.Sprintf("msg=\"failed to marshal log line\" err=%q", err)
		}
		return string(encoded)
	default:
		parts := make([]string, 0, len(keyvals)/2)
		for i := 0; i < len(keyvals); i += 2 {
			key := fmt.Sprint(keyvals[i])
			val := fmt.Sprint(keyvals[i+1])
			parts = append(parts, fmt.Sprintf("%s=%s", key, val))
		}
		return strings.Join(parts, " ")
	}
}
