// SPDX-FileCopyrightText: 2026 Free Mobile
// SPDX-License-Identifier: AGPL-3.0-only

package httpserver

import (
	"fmt"
	"net/http"
)

// Middleware is an HTTP middleware wrapping an http.Handler.
type Middleware = func(http.Handler) http.Handler

// Router is a small wrapper around http.ServeMux that supports per-method
// routing with path parameters (Go 1.22+ patterns), route groups and middleware
// chains.
type Router struct {
	mux         *http.ServeMux
	prefix      string
	middlewares []Middleware
}

// NewRouter creates a new Router.
func NewRouter() *Router {
	return &Router{mux: http.NewServeMux()}
}

// ServeHTTP implements http.Handler.
func (r *Router) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	r.mux.ServeHTTP(w, req)
}

// Use appends middlewares that will be applied to subsequent routes registered
// through this router.
func (r *Router) Use(mw ...Middleware) {
	r.middlewares = append(r.middlewares, mw...)
}

// Group returns a derived Router. All routes registered on the returned Router
// are prefixed with the given prefix and wrapped with the additional
// middlewares.
func (r *Router) Group(prefix string, mw ...Middleware) *Router {
	combined := make([]Middleware, 0, len(r.middlewares)+len(mw))
	combined = append(combined, r.middlewares...)
	combined = append(combined, mw...)
	return &Router{
		mux:         r.mux,
		prefix:      r.prefix + prefix,
		middlewares: combined,
	}
}

// Handle registers handler for the given method and pattern.
func (r *Router) Handle(method, pattern string, handler http.Handler, mw ...Middleware) {
	final := handler
	all := make([]Middleware, 0, len(r.middlewares)+len(mw))
	all = append(all, r.middlewares...)
	all = append(all, mw...)
	for i := len(all) - 1; i >= 0; i-- {
		final = all[i](final)
	}
	r.mux.Handle(fmt.Sprintf("%s %s%s", method, r.prefix, pattern), final)
}

// HandleFunc registers a handler function for the given method and pattern.
func (r *Router) HandleFunc(method, pattern string, handler http.HandlerFunc, mw ...Middleware) {
	r.Handle(method, pattern, handler, mw...)
}

// GET registers a GET handler.
func (r *Router) GET(pattern string, handler http.HandlerFunc, mw ...Middleware) {
	r.Handle(http.MethodGet, pattern, handler, mw...)
}

// POST registers a POST handler.
func (r *Router) POST(pattern string, handler http.HandlerFunc, mw ...Middleware) {
	r.Handle(http.MethodPost, pattern, handler, mw...)
}

// DELETE registers a DELETE handler.
func (r *Router) DELETE(pattern string, handler http.HandlerFunc, mw ...Middleware) {
	r.Handle(http.MethodDelete, pattern, handler, mw...)
}
