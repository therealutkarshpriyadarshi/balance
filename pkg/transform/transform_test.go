package transform

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestTransformer_RequestHeaderTransforms(t *testing.T) {
	config := TransformConfig{
		RequestHeaders: []HeaderTransform{
			{Action: "add", Name: "X-Custom-Header", Value: "value1"},
			{Action: "set", Name: "User-Agent", Value: "Balance/1.0"},
			{Action: "remove", Name: "X-Remove-Me"},
		},
	}

	transformer := NewTransformer(config)

	req := httptest.NewRequest("GET", "http://example.com/test", nil)
	req.Header.Set("X-Remove-Me", "should-be-removed")
	req.Header.Set("User-Agent", "Original/1.0")

	err := transformer.TransformRequest(req)
	if err != nil {
		t.Fatalf("TransformRequest failed: %v", err)
	}

	// Check added header
	if got := req.Header.Get("X-Custom-Header"); got != "value1" {
		t.Errorf("Expected X-Custom-Header=value1, got %s", got)
	}

	// Check set header
	if got := req.Header.Get("User-Agent"); got != "Balance/1.0" {
		t.Errorf("Expected User-Agent=Balance/1.0, got %s", got)
	}

	// Check removed header
	if got := req.Header.Get("X-Remove-Me"); got != "" {
		t.Errorf("Expected X-Remove-Me to be removed, got %s", got)
	}
}

func TestTransformer_ResponseHeaderTransforms(t *testing.T) {
	config := TransformConfig{
		ResponseHeaders: []HeaderTransform{
			{Action: "add", Name: "X-Served-By", Value: "Balance"},
			{Action: "set", Name: "Server", Value: "Balance/1.0"},
			{Action: "remove", Name: "X-Backend-Info"},
		},
	}

	transformer := NewTransformer(config)

	resp := &http.Response{
		Header: make(http.Header),
	}
	resp.Header.Set("X-Backend-Info", "backend-1")
	resp.Header.Set("Server", "Apache/2.4")

	err := transformer.TransformResponse(resp)
	if err != nil {
		t.Fatalf("TransformResponse failed: %v", err)
	}

	// Check added header
	if got := resp.Header.Get("X-Served-By"); got != "Balance" {
		t.Errorf("Expected X-Served-By=Balance, got %s", got)
	}

	// Check set header
	if got := resp.Header.Get("Server"); got != "Balance/1.0" {
		t.Errorf("Expected Server=Balance/1.0, got %s", got)
	}

	// Check removed header
	if got := resp.Header.Get("X-Backend-Info"); got != "" {
		t.Errorf("Expected X-Backend-Info to be removed, got %s", got)
	}
}

func TestTransformer_StripPrefix(t *testing.T) {
	config := TransformConfig{
		StripPrefix: "/api",
	}

	transformer := NewTransformer(config)

	tests := []struct {
		input    string
		expected string
	}{
		{"/api/users", "/users"},
		{"/api/v1/users", "/v1/users"},
		{"/users", "/users"},
		{"/api", "/"},
	}

	for _, tt := range tests {
		req := httptest.NewRequest("GET", "http://example.com"+tt.input, nil)
		err := transformer.TransformRequest(req)
		if err != nil {
			t.Fatalf("TransformRequest failed: %v", err)
		}

		if req.URL.Path != tt.expected {
			t.Errorf("StripPrefix(%s) = %s, expected %s", tt.input, req.URL.Path, tt.expected)
		}
	}
}

func TestTransformer_AddPrefix(t *testing.T) {
	config := TransformConfig{
		AddPrefix: "/v2",
	}

	transformer := NewTransformer(config)

	tests := []struct {
		input    string
		expected string
	}{
		{"/users", "/v2/users"},
		{"/api/users", "/v2/api/users"},
		{"/v2/users", "/v2/users"}, // Already has prefix
	}

	for _, tt := range tests {
		req := httptest.NewRequest("GET", "http://example.com"+tt.input, nil)
		err := transformer.TransformRequest(req)
		if err != nil {
			t.Fatalf("TransformRequest failed: %v", err)
		}

		if req.URL.Path != tt.expected {
			t.Errorf("AddPrefix(%s) = %s, expected %s", tt.input, req.URL.Path, tt.expected)
		}
	}
}

func TestTransformer_PathTransforms(t *testing.T) {
	config := TransformConfig{
		PathTransforms: []PathTransform{
			{Type: "prefix", Pattern: "/old", Replacement: "/new"},
			{Type: "exact", Pattern: "/exact", Replacement: "/replaced"},
		},
	}

	transformer := NewTransformer(config)

	tests := []struct {
		input    string
		expected string
	}{
		{"/old/path", "/new/path"},
		{"/old", "/new"},
		{"/exact", "/replaced"},
		{"/other", "/other"},
	}

	for _, tt := range tests {
		req := httptest.NewRequest("GET", "http://example.com"+tt.input, nil)
		err := transformer.TransformRequest(req)
		if err != nil {
			t.Fatalf("TransformRequest failed: %v", err)
		}

		if req.URL.Path != tt.expected {
			t.Errorf("PathTransform(%s) = %s, expected %s", tt.input, req.URL.Path, tt.expected)
		}
	}
}

func TestAddForwardedHeaders(t *testing.T) {
	req := httptest.NewRequest("GET", "http://example.com/test", nil)
	clientIP := "192.168.1.100"

	AddForwardedHeaders(req, clientIP)

	// Check X-Forwarded-For
	if got := req.Header.Get("X-Forwarded-For"); got != clientIP {
		t.Errorf("Expected X-Forwarded-For=%s, got %s", clientIP, got)
	}

	// Check X-Forwarded-Proto
	if got := req.Header.Get("X-Forwarded-Proto"); got != "http" {
		t.Errorf("Expected X-Forwarded-Proto=http, got %s", got)
	}

	// Check X-Real-IP
	if got := req.Header.Get("X-Real-IP"); got != clientIP {
		t.Errorf("Expected X-Real-IP=%s, got %s", clientIP, got)
	}
}

func TestAddForwardedHeaders_ExistingXFF(t *testing.T) {
	req := httptest.NewRequest("GET", "http://example.com/test", nil)
	req.Header.Set("X-Forwarded-For", "10.0.0.1")
	clientIP := "192.168.1.100"

	AddForwardedHeaders(req, clientIP)

	expected := "10.0.0.1, 192.168.1.100"
	if got := req.Header.Get("X-Forwarded-For"); got != expected {
		t.Errorf("Expected X-Forwarded-For=%s, got %s", expected, got)
	}
}

func TestStripHopByHopHeaders(t *testing.T) {
	header := make(http.Header)
	header.Set("Connection", "keep-alive")
	header.Set("Keep-Alive", "timeout=5")
	header.Set("Proxy-Authorization", "Basic xyz")
	header.Set("Transfer-Encoding", "chunked")
	header.Set("Content-Type", "application/json")

	StripHopByHopHeaders(header)

	// Hop-by-hop headers should be removed
	hopByHop := []string{"Connection", "Keep-Alive", "Proxy-Authorization", "Transfer-Encoding"}
	for _, h := range hopByHop {
		if got := header.Get(h); got != "" {
			t.Errorf("Expected %s to be removed, got %s", h, got)
		}
	}

	// End-to-end headers should remain
	if got := header.Get("Content-Type"); got != "application/json" {
		t.Errorf("Expected Content-Type to remain, got %s", got)
	}
}

func TestRewritePath(t *testing.T) {
	tests := []struct {
		original    string
		pattern     string
		replacement string
		expected    string
	}{
		{"/api/users", "/api", "/v1", "/v1/users"},
		{"/old/path", "/old", "/new", "/new/path"},
		{"/users", "/api", "/v1", "/users"},
	}

	for _, tt := range tests {
		req := httptest.NewRequest("GET", "http://example.com"+tt.original, nil)
		RewritePath(req, tt.pattern, tt.replacement)

		if req.URL.Path != tt.expected {
			t.Errorf("RewritePath(%s, %s, %s) = %s, expected %s",
				tt.original, tt.pattern, tt.replacement, req.URL.Path, tt.expected)
		}
	}
}

func TestStripPrefix_Function(t *testing.T) {
	tests := []struct {
		path     string
		prefix   string
		expected string
	}{
		{"/api/users", "/api", "/users"},
		{"/api/v1/users", "/api", "/v1/users"},
		{"/users", "/api", "/users"},
		{"/api", "/api", "/"},
	}

	for _, tt := range tests {
		req := httptest.NewRequest("GET", "http://example.com"+tt.path, nil)
		StripPrefix(req, tt.prefix)

		if req.URL.Path != tt.expected {
			t.Errorf("StripPrefix(%s, %s) = %s, expected %s",
				tt.path, tt.prefix, req.URL.Path, tt.expected)
		}
	}
}

func TestAddPrefix_Function(t *testing.T) {
	tests := []struct {
		path     string
		prefix   string
		expected string
	}{
		{"/users", "/api", "/api/users"},
		{"/v1/users", "/api", "/api/v1/users"},
		{"/api/users", "/api", "/api/users"},
	}

	for _, tt := range tests {
		req := httptest.NewRequest("GET", "http://example.com"+tt.path, nil)
		AddPrefix(req, tt.prefix)

		if req.URL.Path != tt.expected {
			t.Errorf("AddPrefix(%s, %s) = %s, expected %s",
				tt.path, tt.prefix, req.URL.Path, tt.expected)
		}
	}
}

func TestCopyHeaders(t *testing.T) {
	src := make(http.Header)
	src.Set("Content-Type", "application/json")
	src.Add("X-Custom", "value1")
	src.Add("X-Custom", "value2")

	dst := make(http.Header)
	CopyHeaders(dst, src)

	if got := dst.Get("Content-Type"); got != "application/json" {
		t.Errorf("Expected Content-Type=application/json, got %s", got)
	}

	values := dst["X-Custom"]
	if len(values) != 2 {
		t.Errorf("Expected 2 X-Custom values, got %d", len(values))
	}
}

func TestNormalizeHeaders(t *testing.T) {
	header := make(http.Header)
	header["Content-Type"] = []string{"application/json"}
	header["X-Custom-Header"] = []string{"value"}
	header["UPPERCASE"] = []string{"test"}

	NormalizeHeaders(header)

	// All keys should be lowercase
	for key := range header {
		if key != strings.ToLower(key) {
			t.Errorf("Expected lowercase key, got %s", key)
		}
	}

	// Values should be preserved
	if got := header.Get("content-type"); got != "application/json" {
		t.Errorf("Expected content-type value preserved, got %s", got)
	}
}
