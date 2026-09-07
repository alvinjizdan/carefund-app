package logger

import (
	"context"
	"fmt"
	"io"
	"log"
	"os"
	"strings"
	"sync"
)

type Field struct {
	Key   string
	Value any
}

// F creates a structured logging Field.
func F(key string, value any) Field {
	return Field{Key: key, Value: value}
}

type contextKey string

const requestIDCtxKey = contextKey("request_id")

// ContextWithRequestID attaches request_id to context for correlated logging.
func ContextWithRequestID(ctx context.Context, reqID string) context.Context {
	return context.WithValue(ctx, requestIDCtxKey, reqID)
}

// GetRequestID extracts the request_id from context if available.
func GetRequestID(ctx context.Context) string {
	if ctx == nil {
		return ""
	}
	if v, ok := ctx.Value(requestIDCtxKey).(string); ok && v != "" {
		return v
	}
	if v, ok := ctx.Value("request_id").(string); ok && v != "" {
		return v
	}
	return ""
}

var sensitiveKeyFragments = []string{
	"password",
	"secret",
	"jwt",
	"auth",
	"token",
	"server_key",
	"client_key",
	"private_key",
}

// sanitizeValue ensures credentials, tokens, and secrets are never emitted to logs.
func sanitizeValue(key string, value any) any {
	lowerKey := strings.ToLower(key)
	if lowerKey == "idempotency_key" || lowerKey == "refund_key" {
		return value
	}
	for _, fragment := range sensitiveKeyFragments {
		if strings.Contains(lowerKey, fragment) {
			return "[REDACTED]"
		}
	}
	return value
}

var (
	defaultLogger = log.New(os.Stdout, "", log.Ldate|log.Ltime|log.Lmicroseconds|log.LUTC)
	mu            sync.Mutex
)

// SetOutput redirects logger output (useful for tests).
func SetOutput(w io.Writer) {
	mu.Lock()
	defer mu.Unlock()
	defaultLogger.SetOutput(w)
}

func formatLog(ctx context.Context, level, msg string, err error, fields []Field) string {
	var sb strings.Builder

	sb.WriteString("[")
	sb.WriteString(level)
	sb.WriteString("] ")

	reqID := GetRequestID(ctx)
	if reqID != "" {
		sb.WriteString("[req_id=")
		sb.WriteString(reqID)
		sb.WriteString("] ")
	}

	var component string
	var otherFields []Field
	for _, f := range fields {
		if strings.EqualFold(f.Key, "component") {
			component = fmt.Sprintf("%v", f.Value)
		} else {
			otherFields = append(otherFields, f)
		}
	}

	if component != "" {
		sb.WriteString("[component=")
		sb.WriteString(component)
		sb.WriteString("] ")
	}

	sb.WriteString(msg)

	if err != nil {
		sb.WriteString(" err=")
		sb.WriteString(fmt.Sprintf("%q", err.Error()))
	}

	for _, f := range otherFields {
		sb.WriteString(" ")
		sb.WriteString(f.Key)
		sb.WriteString("=")
		val := sanitizeValue(f.Key, f.Value)
		if s, ok := val.(string); ok && strings.ContainsAny(s, " \t\n\"") {
			sb.WriteString(fmt.Sprintf("%q", s))
		} else {
			sb.WriteString(fmt.Sprintf("%v", val))
		}
	}

	return sb.String()
}

func Info(ctx context.Context, msg string, fields ...Field) {
	mu.Lock()
	defer mu.Unlock()
	defaultLogger.Println(formatLog(ctx, "INFO", msg, nil, fields))
}

func Warn(ctx context.Context, msg string, fields ...Field) {
	mu.Lock()
	defer mu.Unlock()
	defaultLogger.Println(formatLog(ctx, "WARN", msg, nil, fields))
}

func Error(ctx context.Context, msg string, err error, fields ...Field) {
	mu.Lock()
	defer mu.Unlock()
	defaultLogger.Println(formatLog(ctx, "ERROR", msg, err, fields))
}

func Fatal(ctx context.Context, msg string, err error, fields ...Field) {
	mu.Lock()
	defer mu.Unlock()
	defaultLogger.Println(formatLog(ctx, "FATAL", msg, err, fields))
	os.Exit(1)
}
