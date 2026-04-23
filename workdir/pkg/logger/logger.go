// Package logger provides a minimal JSON-structured logger compatible with Go 1.18.
// Fields are written as key=value pairs on a single line for easy grep / log aggregation.
package logger

import (
	"context"
	"fmt"
	"log"
	"os"
	"strings"
	"time"
)

// contextKey is unexported to prevent collisions across packages.
type contextKey int

const (
	keyRequestID contextKey = iota
	keyOrgID
	keyUsername
)

// WithRequestID returns a context carrying the given request ID.
func WithRequestID(ctx context.Context, id string) context.Context {
	return context.WithValue(ctx, keyRequestID, id)
}

// WithOrgID returns a context carrying the given org ID.
func WithOrgID(ctx context.Context, id string) context.Context {
	return context.WithValue(ctx, keyOrgID, id)
}

// WithUsername returns a context carrying the authenticated username.
func WithUsername(ctx context.Context, name string) context.Context {
	return context.WithValue(ctx, keyUsername, name)
}

// RequestID retrieves the request ID from a context (empty string if absent).
func RequestID(ctx context.Context) string { return strVal(ctx, keyRequestID) }

// OrgID retrieves the org ID from a context.
func OrgID(ctx context.Context) string { return strVal(ctx, keyOrgID) }

// Username retrieves the username from a context.
func Username(ctx context.Context) string { return strVal(ctx, keyUsername) }

func strVal(ctx context.Context, k contextKey) string {
	if v, ok := ctx.Value(k).(string); ok {
		return v
	}
	return ""
}

// Logger is a structured logger that emits key=value lines to stderr.
type Logger struct {
	component string
	l         *log.Logger
}

// New returns a Logger for the named component.
func New(component string) *Logger {
	return &Logger{
		component: component,
		l:         log.New(os.Stderr, "", 0),
	}
}

// Default is the application-wide logger.
var Default = New("ohe")

func (lg *Logger) emit(level, msg string, ctx context.Context, kvs ...interface{}) {
	var b strings.Builder
	b.WriteString("ts=")
	b.WriteString(time.Now().UTC().Format(time.RFC3339))
	b.WriteString(" level=")
	b.WriteString(level)
	b.WriteString(" component=")
	b.WriteString(lg.component)

	if ctx != nil {
		if rid := RequestID(ctx); rid != "" {
			b.WriteString(" request_id=")
			b.WriteString(rid)
		}
		if oid := OrgID(ctx); oid != "" {
			b.WriteString(" org_id=")
			b.WriteString(oid)
		}
		if u := Username(ctx); u != "" {
			b.WriteString(" user=")
			b.WriteString(u)
		}
	}

	b.WriteString(" msg=")
	b.WriteString(quoteIfNeeded(msg))

	for i := 0; i+1 < len(kvs); i += 2 {
		b.WriteByte(' ')
		_, _ = fmt.Fprint(&b, kvs[i])
		b.WriteByte('=')
		b.WriteString(quoteIfNeeded(fmt.Sprint(kvs[i+1])))
	}

	lg.l.Println(b.String())
}

func quoteIfNeeded(s string) string {
	if strings.ContainsAny(s, " \t\n\"") {
		return fmt.Sprintf("%q", s)
	}
	return s
}

// Info logs an informational message.
func (lg *Logger) Info(msg string, kvs ...interface{}) { lg.emit("info", msg, nil, kvs...) }

// InfoCtx logs an informational message with context fields.
func (lg *Logger) InfoCtx(ctx context.Context, msg string, kvs ...interface{}) {
	lg.emit("info", msg, ctx, kvs...)
}

// Warn logs a warning.
func (lg *Logger) Warn(msg string, kvs ...interface{}) { lg.emit("warn", msg, nil, kvs...) }

// WarnCtx logs a warning with context fields.
func (lg *Logger) WarnCtx(ctx context.Context, msg string, kvs ...interface{}) {
	lg.emit("warn", msg, ctx, kvs...)
}

// Error logs an error.
func (lg *Logger) Error(msg string, kvs ...interface{}) { lg.emit("error", msg, nil, kvs...) }

// ErrorCtx logs an error with context fields.
func (lg *Logger) ErrorCtx(ctx context.Context, msg string, kvs ...interface{}) {
	lg.emit("error", msg, ctx, kvs...)
}

// Debug logs a debug message (omitted in production — controlled by LOG_LEVEL env var).
func (lg *Logger) Debug(msg string, kvs ...interface{}) {
	if logLevel() == "debug" {
		lg.emit("debug", msg, nil, kvs...)
	}
}

// DebugCtx logs a debug message with context fields.
func (lg *Logger) DebugCtx(ctx context.Context, msg string, kvs ...interface{}) {
	if logLevel() == "debug" {
		lg.emit("debug", msg, ctx, kvs...)
	}
}

func logLevel() string {
	lv := strings.ToLower(os.Getenv("LOG_LEVEL"))
	if lv == "" {
		return "info"
	}
	return lv
}
