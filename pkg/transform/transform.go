package transform

import (
	"net/http"
	"strings"
)

// HeaderTransform defines a header transformation
type HeaderTransform struct {
	Action string // "add", "set", "remove"
	Name   string
	Value  string
}

// PathTransform defines a path transformation
type PathTransform struct {
	Type        string // "prefix", "regex", "exact"
	Pattern     string
	Replacement string
}

// TransformConfig configures request/response transformations
type TransformConfig struct {
	RequestHeaders  []HeaderTransform
	ResponseHeaders []HeaderTransform
	PathTransforms  []PathTransform
	StripPrefix     string
	AddPrefix       string
}

// Transformer handles request and response transformations
type Transformer struct {
	config TransformConfig
}

// NewTransformer creates a new transformer
func NewTransformer(config TransformConfig) *Transformer {
	return &Transformer{
		config: config,
	}
}

// TransformRequest applies transformations to an HTTP request
func (t *Transformer) TransformRequest(req *http.Request) error {
	// Apply header transformations
	for _, transform := range t.config.RequestHeaders {
		switch transform.Action {
		case "add":
			req.Header.Add(transform.Name, transform.Value)
		case "set":
			req.Header.Set(transform.Name, transform.Value)
		case "remove":
			req.Header.Del(transform.Name)
		}
	}

	// Apply path transformations
	if t.config.StripPrefix != "" {
		req.URL.Path = strings.TrimPrefix(req.URL.Path, t.config.StripPrefix)
		if !strings.HasPrefix(req.URL.Path, "/") {
			req.URL.Path = "/" + req.URL.Path
		}
	}

	if t.config.AddPrefix != "" {
		if !strings.HasPrefix(req.URL.Path, t.config.AddPrefix) {
			req.URL.Path = t.config.AddPrefix + req.URL.Path
		}
	}

	// Apply path pattern transformations
	for _, pt := range t.config.PathTransforms {
		switch pt.Type {
		case "prefix":
			if strings.HasPrefix(req.URL.Path, pt.Pattern) {
				req.URL.Path = strings.Replace(req.URL.Path, pt.Pattern, pt.Replacement, 1)
			}
		case "exact":
			if req.URL.Path == pt.Pattern {
				req.URL.Path = pt.Replacement
			}
		}
	}

	return nil
}

// TransformResponse applies transformations to an HTTP response
func (t *Transformer) TransformResponse(resp *http.Response) error {
	// Apply header transformations
	for _, transform := range t.config.ResponseHeaders {
		switch transform.Action {
		case "add":
			resp.Header.Add(transform.Name, transform.Value)
		case "set":
			resp.Header.Set(transform.Name, transform.Value)
		case "remove":
			resp.Header.Del(transform.Name)
		}
	}

	return nil
}

// AddRequestHeader adds a header to the request
func AddRequestHeader(req *http.Request, name, value string) {
	req.Header.Add(name, value)
}

// SetRequestHeader sets a header on the request
func SetRequestHeader(req *http.Request, name, value string) {
	req.Header.Set(name, value)
}

// RemoveRequestHeader removes a header from the request
func RemoveRequestHeader(req *http.Request, name string) {
	req.Header.Del(name)
}

// AddResponseHeader adds a header to the response
func AddResponseHeader(resp *http.Response, name, value string) {
	resp.Header.Add(name, value)
}

// SetResponseHeader sets a header on the response
func SetResponseHeader(resp *http.Response, name, value string) {
	resp.Header.Set(name, value)
}

// RemoveResponseHeader removes a header from the response
func RemoveResponseHeader(resp *http.Response, name string) {
	resp.Header.Del(name)
}

// AddForwardedHeaders adds standard forwarding headers
func AddForwardedHeaders(req *http.Request, clientIP string) {
	// Add X-Forwarded-For
	if prior := req.Header.Get("X-Forwarded-For"); prior != "" {
		clientIP = prior + ", " + clientIP
	}
	req.Header.Set("X-Forwarded-For", clientIP)

	// Add X-Forwarded-Proto
	if req.TLS != nil {
		req.Header.Set("X-Forwarded-Proto", "https")
	} else {
		req.Header.Set("X-Forwarded-Proto", "http")
	}

	// Add X-Forwarded-Host
	if req.Host != "" {
		req.Header.Set("X-Forwarded-Host", req.Host)
	}

	// Add X-Real-IP
	req.Header.Set("X-Real-IP", clientIP)
}

// StripHopByHopHeaders removes hop-by-hop headers
func StripHopByHopHeaders(header http.Header) {
	// HTTP/1.1 hop-by-hop headers
	hopByHopHeaders := []string{
		"Connection",
		"Keep-Alive",
		"Proxy-Authenticate",
		"Proxy-Authorization",
		"Te",
		"Trailers",
		"Transfer-Encoding",
		"Upgrade",
	}

	for _, h := range hopByHopHeaders {
		header.Del(h)
	}

	// Remove headers specified in Connection header
	if connection := header.Get("Connection"); connection != "" {
		for _, h := range strings.Split(connection, ",") {
			header.Del(strings.TrimSpace(h))
		}
	}
}

// RewritePath rewrites the request path
func RewritePath(req *http.Request, pattern, replacement string) {
	req.URL.Path = strings.Replace(req.URL.Path, pattern, replacement, 1)
}

// StripPrefix removes a prefix from the request path
func StripPrefix(req *http.Request, prefix string) {
	req.URL.Path = strings.TrimPrefix(req.URL.Path, prefix)
	if !strings.HasPrefix(req.URL.Path, "/") {
		req.URL.Path = "/" + req.URL.Path
	}
}

// AddPrefix adds a prefix to the request path
func AddPrefix(req *http.Request, prefix string) {
	if !strings.HasPrefix(req.URL.Path, prefix) {
		req.URL.Path = prefix + req.URL.Path
	}
}

// CopyHeaders copies headers from src to dst
func CopyHeaders(dst, src http.Header) {
	for key, values := range src {
		for _, value := range values {
			dst.Add(key, value)
		}
	}
}

// NormalizeHeaders normalizes header names
func NormalizeHeaders(header http.Header) {
	// HTTP/2 requires lowercase header names
	for key := range header {
		if key != strings.ToLower(key) {
			values := header[key]
			delete(header, key)
			header[strings.ToLower(key)] = values
		}
	}
}
