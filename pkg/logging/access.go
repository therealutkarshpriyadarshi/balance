package logging

import (
	"bufio"
	"fmt"
	"net"
	"net/http"
	"time"
)

// AccessLog represents an HTTP access log entry
type AccessLog struct {
	Timestamp      time.Time
	ClientIP       string
	Method         string
	Path           string
	Query          string
	Protocol       string
	StatusCode     int
	BytesWritten   int64
	Duration       time.Duration
	UserAgent      string
	Referer        string
	Backend        string
	TraceID        string
	RequestHeaders map[string]string
}

// AccessLogger logs HTTP access
type AccessLogger struct {
	logger *Logger
}

// NewAccessLogger creates a new access logger
func NewAccessLogger(logger *Logger) *AccessLogger {
	return &AccessLogger{
		logger: logger,
	}
}

// Log logs an access entry
func (al *AccessLogger) Log(entry AccessLog) {
	fields := []Field{
		String("client_ip", entry.ClientIP),
		String("method", entry.Method),
		String("path", entry.Path),
		String("protocol", entry.Protocol),
		Int("status", entry.StatusCode),
		Int64("bytes", entry.BytesWritten),
		Duration("duration", entry.Duration),
		String("user_agent", entry.UserAgent),
	}

	if entry.Query != "" {
		fields = append(fields, String("query", entry.Query))
	}

	if entry.Referer != "" {
		fields = append(fields, String("referer", entry.Referer))
	}

	if entry.Backend != "" {
		fields = append(fields, String("backend", entry.Backend))
	}

	if entry.TraceID != "" {
		fields = append(fields, String("trace_id", entry.TraceID))
	}

	al.logger.Info("access", fields...)
}

// AccessLogMiddleware creates middleware for access logging
func AccessLogMiddleware(logger *Logger) func(http.Handler) http.Handler {
	accessLogger := NewAccessLogger(logger)

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()

			// Wrap response writer to capture status and bytes
			lrw := &loggingResponseWriter{
				ResponseWriter: w,
				statusCode:     http.StatusOK,
				bytesWritten:   0,
			}

			// Handle request
			next.ServeHTTP(lrw, r)

			// Extract client IP
			clientIP := r.RemoteAddr
			if forwarded := r.Header.Get("X-Forwarded-For"); forwarded != "" {
				clientIP = forwarded
			}

			// Create access log entry
			entry := AccessLog{
				Timestamp:    start,
				ClientIP:     clientIP,
				Method:       r.Method,
				Path:         r.URL.Path,
				Query:        r.URL.RawQuery,
				Protocol:     r.Proto,
				StatusCode:   lrw.statusCode,
				BytesWritten: lrw.bytesWritten,
				Duration:     time.Since(start),
				UserAgent:    r.UserAgent(),
				Referer:      r.Referer(),
			}

			accessLogger.Log(entry)
		})
	}
}

// loggingResponseWriter wraps http.ResponseWriter to capture status and bytes
type loggingResponseWriter struct {
	http.ResponseWriter
	statusCode   int
	bytesWritten int64
}

func (lrw *loggingResponseWriter) WriteHeader(code int) {
	lrw.statusCode = code
	lrw.ResponseWriter.WriteHeader(code)
}

func (lrw *loggingResponseWriter) Write(b []byte) (int, error) {
	n, err := lrw.ResponseWriter.Write(b)
	lrw.bytesWritten += int64(n)
	return n, err
}

// Flush implements http.Flusher if the underlying ResponseWriter does
func (lrw *loggingResponseWriter) Flush() {
	if flusher, ok := lrw.ResponseWriter.(http.Flusher); ok {
		flusher.Flush()
	}
}

// Hijack implements http.Hijacker if the underlying ResponseWriter does
func (lrw *loggingResponseWriter) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	if hijacker, ok := lrw.ResponseWriter.(http.Hijacker); ok {
		return hijacker.Hijack()
	}
	return nil, nil, fmt.Errorf("ResponseWriter does not implement http.Hijacker")
}
