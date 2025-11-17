package logging

import (
	"context"
	"fmt"
	"io"
	"os"
	"runtime"
	"strings"
	"sync"
	"time"

	"go.opentelemetry.io/otel/trace"
)

// Level represents logging level
type Level int

const (
	DebugLevel Level = iota
	InfoLevel
	WarnLevel
	ErrorLevel
	FatalLevel
)

func (l Level) String() string {
	switch l {
	case DebugLevel:
		return "DEBUG"
	case InfoLevel:
		return "INFO"
	case WarnLevel:
		return "WARN"
	case ErrorLevel:
		return "ERROR"
	case FatalLevel:
		return "FATAL"
	default:
		return "UNKNOWN"
	}
}

// Field represents a log field
type Field struct {
	Key   string
	Value interface{}
}

// String creates a string field
func String(key, value string) Field {
	return Field{Key: key, Value: value}
}

// Int creates an int field
func Int(key string, value int) Field {
	return Field{Key: key, Value: value}
}

// Int64 creates an int64 field
func Int64(key string, value int64) Field {
	return Field{Key: key, Value: value}
}

// Float64 creates a float64 field
func Float64(key string, value float64) Field {
	return Field{Key: key, Value: value}
}

// Bool creates a bool field
func Bool(key string, value bool) Field {
	return Field{Key: key, Value: value}
}

// Err creates an error field
func Err(err error) Field {
	if err == nil {
		return Field{Key: "error", Value: nil}
	}
	return Field{Key: "error", Value: err.Error()}
}

// Duration creates a duration field
func Duration(key string, value time.Duration) Field {
	return Field{Key: key, Value: value.String()}
}

// Any creates a field with any value
func Any(key string, value interface{}) Field {
	return Field{Key: key, Value: value}
}

// Logger provides structured logging
type Logger struct {
	level      Level
	output     io.Writer
	mu         sync.Mutex
	timeFormat string
	addCaller  bool
}

// Config configures the logger
type Config struct {
	Level      Level
	Output     io.Writer
	TimeFormat string
	AddCaller  bool
}

// NewLogger creates a new logger
func NewLogger(config Config) *Logger {
	if config.Output == nil {
		config.Output = os.Stdout
	}
	if config.TimeFormat == "" {
		config.TimeFormat = time.RFC3339
	}
	return &Logger{
		level:      config.Level,
		output:     config.Output,
		timeFormat: config.TimeFormat,
		addCaller:  config.AddCaller,
	}
}

// NewDefaultLogger creates a logger with default settings
func NewDefaultLogger() *Logger {
	return NewLogger(Config{
		Level:      InfoLevel,
		Output:     os.Stdout,
		TimeFormat: time.RFC3339,
		AddCaller:  false,
	})
}

// SetLevel sets the logging level
func (l *Logger) SetLevel(level Level) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.level = level
}

// Debug logs a debug message
func (l *Logger) Debug(msg string, fields ...Field) {
	l.log(DebugLevel, msg, fields...)
}

// Info logs an info message
func (l *Logger) Info(msg string, fields ...Field) {
	l.log(InfoLevel, msg, fields...)
}

// Warn logs a warning message
func (l *Logger) Warn(msg string, fields ...Field) {
	l.log(WarnLevel, msg, fields...)
}

// Error logs an error message
func (l *Logger) Error(msg string, fields ...Field) {
	l.log(ErrorLevel, msg, fields...)
}

// Fatal logs a fatal message and exits
func (l *Logger) Fatal(msg string, fields ...Field) {
	l.log(FatalLevel, msg, fields...)
	os.Exit(1)
}

// DebugContext logs a debug message with context
func (l *Logger) DebugContext(ctx context.Context, msg string, fields ...Field) {
	l.logContext(ctx, DebugLevel, msg, fields...)
}

// InfoContext logs an info message with context
func (l *Logger) InfoContext(ctx context.Context, msg string, fields ...Field) {
	l.logContext(ctx, InfoLevel, msg, fields...)
}

// WarnContext logs a warning message with context
func (l *Logger) WarnContext(ctx context.Context, msg string, fields ...Field) {
	l.logContext(ctx, WarnLevel, msg, fields...)
}

// ErrorContext logs an error message with context
func (l *Logger) ErrorContext(ctx context.Context, msg string, fields ...Field) {
	l.logContext(ctx, ErrorLevel, msg, fields...)
}

func (l *Logger) log(level Level, msg string, fields ...Field) {
	l.logContext(context.Background(), level, msg, fields...)
}

func (l *Logger) logContext(ctx context.Context, level Level, msg string, fields ...Field) {
	l.mu.Lock()
	defer l.mu.Unlock()

	if level < l.level {
		return
	}

	// Build log entry
	var b strings.Builder

	// Timestamp
	b.WriteString(time.Now().Format(l.timeFormat))
	b.WriteString(" ")

	// Level
	b.WriteString(level.String())
	b.WriteString(" ")

	// Trace ID (if available)
	if span := trace.SpanFromContext(ctx); span.SpanContext().IsValid() {
		b.WriteString("trace_id=")
		b.WriteString(span.SpanContext().TraceID().String())
		b.WriteString(" ")
		b.WriteString("span_id=")
		b.WriteString(span.SpanContext().SpanID().String())
		b.WriteString(" ")
	}

	// Caller info
	if l.addCaller {
		if _, file, line, ok := runtime.Caller(3); ok {
			// Get just the filename, not full path
			parts := strings.Split(file, "/")
			filename := parts[len(parts)-1]
			b.WriteString(fmt.Sprintf("%s:%d ", filename, line))
		}
	}

	// Message
	b.WriteString(msg)

	// Fields
	if len(fields) > 0 {
		b.WriteString(" ")
		for i, field := range fields {
			if i > 0 {
				b.WriteString(" ")
			}
			b.WriteString(field.Key)
			b.WriteString("=")
			b.WriteString(fmt.Sprintf("%v", field.Value))
		}
	}

	b.WriteString("\n")

	// Write to output
	l.output.Write([]byte(b.String()))
}

// With creates a child logger with additional fields
func (l *Logger) With(fields ...Field) *Logger {
	// For simplicity, return the same logger
	// In a production system, this would create a child logger with bound fields
	return l
}

// Global logger instance
var globalLogger = NewDefaultLogger()

// SetGlobalLogger sets the global logger
func SetGlobalLogger(logger *Logger) {
	globalLogger = logger
}

// Debug logs a debug message using the global logger
func Debug(msg string, fields ...Field) {
	globalLogger.Debug(msg, fields...)
}

// Info logs an info message using the global logger
func Info(msg string, fields ...Field) {
	globalLogger.Info(msg, fields...)
}

// Warn logs a warning message using the global logger
func Warn(msg string, fields ...Field) {
	globalLogger.Warn(msg, fields...)
}

// Error logs an error message using the global logger
func Error(msg string, fields ...Field) {
	globalLogger.Error(msg, fields...)
}

// Fatal logs a fatal message using the global logger and exits
func Fatal(msg string, fields ...Field) {
	globalLogger.Fatal(msg, fields...)
}

// DebugContext logs a debug message with context using the global logger
func DebugContext(ctx context.Context, msg string, fields ...Field) {
	globalLogger.DebugContext(ctx, msg, fields...)
}

// InfoContext logs an info message with context using the global logger
func InfoContext(ctx context.Context, msg string, fields ...Field) {
	globalLogger.InfoContext(ctx, msg, fields...)
}

// WarnContext logs a warning message with context using the global logger
func WarnContext(ctx context.Context, msg string, fields ...Field) {
	globalLogger.WarnContext(ctx, msg, fields...)
}

// ErrorContext logs an error message with context using the global logger
func ErrorContext(ctx context.Context, msg string, fields ...Field) {
	globalLogger.ErrorContext(ctx, msg, fields...)
}
