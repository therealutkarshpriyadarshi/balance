package router

import (
	"net/http"
	"sort"
	"strings"

	"github.com/therealutkarshpriyadarshi/balance/pkg/backend"
	"github.com/therealutkarshpriyadarshi/balance/pkg/config"
)

// Router handles HTTP request routing
type Router struct {
	routes       []*RouteEntry
	defaultPool  *backend.Pool
}

// RouteEntry represents a compiled route with its backend pool
type RouteEntry struct {
	config  config.Route
	pool    *backend.Pool
}

// NewRouter creates a new HTTP router
func NewRouter(routes []config.Route, allBackends *backend.Pool) *Router {
	r := &Router{
		routes:      make([]*RouteEntry, 0, len(routes)),
		defaultPool: allBackends,
	}

	// Create route entries
	for _, routeCfg := range routes {
		pool := backend.NewPool()

		// Add specified backends to this route's pool
		for _, backendName := range routeCfg.Backends {
			if b := allBackends.GetByName(backendName); b != nil {
				pool.Add(b)
			}
		}

		r.routes = append(r.routes, &RouteEntry{
			config: routeCfg,
			pool:   pool,
		})
	}

	// Sort routes by priority (higher priority first)
	sort.Slice(r.routes, func(i, j int) bool {
		return r.routes[i].config.Priority > r.routes[j].config.Priority
	})

	return r
}

// Match finds the best matching route for the given request
func (r *Router) Match(req *http.Request) *backend.Pool {
	// Try each route in priority order
	for _, route := range r.routes {
		if r.matchRoute(req, &route.config) {
			return route.pool
		}
	}

	// No route matched, use default pool
	return r.defaultPool
}

// matchRoute checks if a request matches a route
func (r *Router) matchRoute(req *http.Request, route *config.Route) bool {
	// Check host matching
	if route.Host != "" {
		if !matchHost(req.Host, route.Host) {
			return false
		}
	}

	// Check path prefix matching
	if route.PathPrefix != "" {
		if !strings.HasPrefix(req.URL.Path, route.PathPrefix) {
			return false
		}
	}

	// Check header matching
	if len(route.Headers) > 0 {
		for key, value := range route.Headers {
			if req.Header.Get(key) != value {
				return false
			}
		}
	}

	return true
}

// matchHost checks if the request host matches the route host pattern
func matchHost(requestHost, routeHost string) bool {
	// Remove port from request host
	if idx := strings.Index(requestHost, ":"); idx != -1 {
		requestHost = requestHost[:idx]
	}

	// Exact match
	if requestHost == routeHost {
		return true
	}

	// Wildcard match (e.g., *.example.com)
	if strings.HasPrefix(routeHost, "*.") {
		suffix := routeHost[1:] // Remove the *
		return strings.HasSuffix(requestHost, suffix)
	}

	return false
}
