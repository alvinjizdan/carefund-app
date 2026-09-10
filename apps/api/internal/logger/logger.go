package logger

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"os"
	"strings"
	"sync"
	"time"
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

func formatJSON(ctx context.Context, level, msg string, err error, fields []Field) string {
	payload := make(map[string]any, len(fields)+5)
	payload["time"] = time.Now().UTC().Format(time.RFC3339Nano)
	payload["level"] = level
	if reqID := GetRequestID(ctx); reqID != "" {
		payload["request_id"] = reqID
	}
	payload["message"] = msg
	if err != nil {
		payload["error"] = err.Error()
	}
	for _, f := range fields {
		payload[f.Key] = sanitizeValue(f.Key, f.Value)
	}
	b, marshalErr := json.Marshal(payload)
	if marshalErr != nil {
		return fmt.Sprintf(`{"time":%q,"level":%q,"message":%q,"marshal_error":%q}`,
			time.Now().UTC().Format(time.RFC3339Nano), level, msg, marshalErr.Error())
	}
	return string(b)
}

func formatLog(ctx context.Context, level, msg string, err error, fields []Field) string {
	if strings.ToLower(os.Getenv("LOG_FORMAT")) == "json" {
		return formatJSON(ctx, level, msg, err, fields)
	}

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

func writeLog(entry string) {
	mu.Lock()
	defer mu.Unlock()
	if strings.ToLower(os.Getenv("LOG_FORMAT")) == "json" {
		fmt.Fprintln(defaultLogger.Writer(), entry)
	} else {
		defaultLogger.Println(entry)
	}
}

func Info(ctx context.Context, msg string, fields ...Field) {
	writeLog(formatLog(ctx, "INFO", msg, nil, fields))
}

func Warn(ctx context.Context, msg string, fields ...Field) {
	writeLog(formatLog(ctx, "WARN", msg, nil, fields))
}

func Error(ctx context.Context, msg string, err error, fields ...Field) {
	writeLog(formatLog(ctx, "ERROR", msg, err, fields))
}

func Fatal(ctx context.Context, msg string, err error, fields ...Field) {
	writeLog(formatLog(ctx, "FATAL", msg, err, fields))
	os.Exit(1)
}
