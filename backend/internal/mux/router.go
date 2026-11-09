package mux

import (
	"context"
	"net/http"
	"strings"
)

// Router is a small, dependency-free HTTP mux supporting static paths and
// single-segment parameters (e.g. /users/:id). It is intentionally minimal so
// the application builds with the Go standard library only.
type Router struct {
	routes   []route
	notFound http.Handler
}

type route struct {
	method  string
	path    string
	handler http.HandlerFunc
}

type paramsKey struct{}

// Params returns route parameters captured from the request context.
func Params(req *http.Request) map[string]string {
	if p, ok := req.Context().Value(paramsKey{}).(map[string]string); ok {
		return p
	}
	return nil
}

func NewRouter() *Router {
	return &Router{}
}

// SetNotFoundHandler registers a fallback handler that is invoked when no
// route matches the request. It is used to serve the SPA for client-side
// routes while keeping unknown API paths as API responses.
func (r *Router) SetNotFoundHandler(handler http.Handler) {
	r.notFound = handler
}

func (r *Router) Handle(method, path string, handler http.HandlerFunc) {
	r.routes = append(r.routes, route{method: method, path: cleanPath(path), handler: handler})
}

func (r *Router) GET(path string, handler http.HandlerFunc) { r.Handle(http.MethodGet, path, handler) }
func (r *Router) POST(path string, handler http.HandlerFunc) {
	r.Handle(http.MethodPost, path, handler)
}
func (r *Router) PUT(path string, handler http.HandlerFunc) { r.Handle(http.MethodPut, path, handler) }
func (r *Router) DELETE(path string, handler http.HandlerFunc) {
	r.Handle(http.MethodDelete, path, handler)
}

func cleanPath(p string) string {
	if p == "" {
		return "/"
	}
	if !strings.HasPrefix(p, "/") {
		p = "/" + p
	}
	p = strings.TrimSuffix(p, "/")
	if p == "" {
		p = "/"
	}
	return p
}

func (r *Router) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	path := cleanPath(req.URL.Path)

	pathMatched := false
	var allowed []string
	for _, route := range r.routes {
		params, ok := match(route.path, path)
		if !ok {
			continue
		}
		pathMatched = true
		allowed = append(allowed, route.method)
		if route.method != req.Method {
			continue
		}
		if len(params) > 0 {
			req = req.WithContext(context.WithValue(req.Context(), paramsKey{}, params))
		}
		route.handler(w, req)
		return
	}

	if pathMatched {
		w.Header().Set("Allow", strings.Join(allowed, ", "))
		http.Error(w, `{"error":"method not allowed"}`, http.StatusMethodNotAllowed)
		return
	}

	if r.notFound != nil {
		r.notFound.ServeHTTP(w, req)
		return
	}
	http.NotFound(w, req)
}

func match(pattern, path string) (map[string]string, bool) {
	pSegs := splitPath(pattern)
	qSegs := splitPath(path)
	if len(pSegs) != len(qSegs) {
		return nil, false
	}
	params := make(map[string]string)
	for i := range pSegs {
		if strings.HasPrefix(pSegs[i], ":") {
			params[strings.TrimPrefix(pSegs[i], ":")] = qSegs[i]
			continue
		}
		if pSegs[i] != qSegs[i] {
			return nil, false
		}
	}
	return params, true
}

func splitPath(p string) []string {
	p = strings.Trim(p, "/")
	if p == "" {
		return []string{}
	}
	return strings.Split(p, "/")
}
