package http

import (
	"encoding/json"
	"net/http"
	"strings"
	"sync"
)

// HandlerFunc is the primary handler signature for JIMO.
type HandlerFunc func(*Context)

// Middleware wraps a handler with additional behavior.
//
// It follows a functional style: the middleware receives the next handler and returns a new handler.
type Middleware func(next HandlerFunc) HandlerFunc

type routeOptions struct {
	name       string
	middleware []Middleware
}

// RouteOption configures per-route behavior (named routes, middleware, ...).
type RouteOption func(*routeOptions)

// Named assigns a name to a route.
func Named(name string) RouteOption {
	return func(o *routeOptions) {
		o.name = name
	}
}

// WithMiddleware attaches middleware to a single route.
func WithMiddleware(mw ...Middleware) RouteOption {
	return func(o *routeOptions) {
		o.middleware = append(o.middleware, mw...)
	}
}

type routeNode struct {
	static    map[string]*routeNode
	param     *routeNode
	paramName string
	handler   HandlerFunc
	mw        []Middleware
	name      string
}

type routerState struct {
	mu    sync.RWMutex
	trees map[string]*routeNode // method -> route tree
	views *viewEngine
	names map[string]string // route name -> pattern
}

// Router is a minimal, expressive HTTP router.
//
// Phase 1 intentionally supports exact-path matching only.
// It is designed so we can later swap its matcher with a radix tree without changing the public API.
type Router struct {
	prefix string
	state  *routerState
	mw     []Middleware
}

// NewRouter creates a new router.
func NewRouter() *Router {
	return &Router{
		state: &routerState{
			trees: make(map[string]*routeNode),
			views: newViewEngine("views"),
			names: make(map[string]string),
		},
	}
}

// SetViewsDir configures the directory used for Context.View().
func (r *Router) SetViewsDir(dir string) {
	r.state.views.SetDir(dir)
}

// Use registers middleware for the current router scope.
//
// When called on the root router, middleware becomes effectively global.
func (r *Router) Use(mw ...Middleware) {
	r.mw = append(r.mw, mw...)
}

// URL returns a route path by its name.
//
// Params are substituted by replacing {key} tokens.
func (r *Router) URL(name string, params map[string]string) string {
	r.state.mu.RLock()
	pattern := r.state.names[name]
	r.state.mu.RUnlock()
	if pattern == "" {
		return ""
	}
	if len(params) == 0 {
		return pattern
	}
	out := pattern
	for k, v := range params {
		out = strings.ReplaceAll(out, "{"+k+"}", v)
	}
	return out
}

// Get registers a GET route.
func (r *Router) Get(path string, handler HandlerFunc, opts ...RouteOption) {
	r.add(http.MethodGet, path, handler, opts...)
}

// Post registers a POST route.
func (r *Router) Post(path string, handler HandlerFunc, opts ...RouteOption) {
	r.add(http.MethodPost, path, handler, opts...)
}

// Group creates a new router scope under prefix.
func (r *Router) Group(prefix string, fn func(r *Router)) {
	if fn == nil {
		return
	}
	child := &Router{prefix: joinPath(r.prefix, prefix), state: r.state, mw: append([]Middleware(nil), r.mw...)}
	fn(child)
}

func (r *Router) add(method, path string, handler HandlerFunc, opts ...RouteOption) {
	if handler == nil {
		panic("router: handler is nil")
	}

	var ro routeOptions
	for _, opt := range opts {
		if opt != nil {
			opt(&ro)
		}
	}

	full := joinPath(r.prefix, path)
	segs := pathSegments(full)

	r.state.mu.Lock()
	defer r.state.mu.Unlock()

	root := r.state.trees[method]
	if root == nil {
		root = &routeNode{static: make(map[string]*routeNode)}
		r.state.trees[method] = root
	}

	n := root
	for _, seg := range segs {
		if name, ok := isParamSegment(seg); ok {
			if n.param == nil {
				n.param = &routeNode{static: make(map[string]*routeNode), paramName: name}
			} else if n.param.paramName != name {
				panic("router: conflicting param name at " + full)
			}
			n = n.param
			continue
		}

		child := n.static[seg]
		if child == nil {
			child = &routeNode{static: make(map[string]*routeNode)}
			n.static[seg] = child
		}
		n = child
	}

	n.handler = handler
	n.mw = append(append([]Middleware(nil), r.mw...), ro.middleware...)
	n.name = ro.name
	if ro.name != "" {
		if existing := r.state.names[ro.name]; existing != "" && existing != full {
			panic("router: duplicate route name " + ro.name)
		}
		r.state.names[ro.name] = full
	}
}

// ServeHTTP implements http.Handler.
func (r *Router) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	path := cleanPath(req.URL.Path)
	method := req.Method
	segs := pathSegments(path)

	r.state.mu.RLock()
	root := r.state.trees[method]
	views := r.state.views
	r.state.mu.RUnlock()

	if root == nil {
		http.NotFound(w, req)
		return
	}

	n := root
	var params map[string]string
	for _, seg := range segs {
		if next := n.static[seg]; next != nil {
			n = next
			continue
		}
		if n.param == nil {
			http.NotFound(w, req)
			return
		}
		if params == nil {
			params = make(map[string]string, 2)
		}
		params[n.param.paramName] = seg
		n = n.param
	}

	h := n.handler
	if h == nil {
		http.NotFound(w, req)
		return
	}

	if len(n.mw) > 0 {
		h = applyMiddleware(h, n.mw)
	}

	ctx := NewContext(w, req, views)
	ctx.params = params

	defer func() {
		if rec := recover(); rec != nil {
			switch v := rec.(type) {
			case HTTPError:
				writeJSONError(w, v.Status, v.Message, v.Err)
			case *HTTPError:
				writeJSONError(w, v.Status, v.Message, v.Err)
			default:
				writeJSONError(w, http.StatusInternalServerError, "Internal Server Error", nil)
			}
		}
	}()

	h(ctx)
}

func applyMiddleware(h HandlerFunc, chain []Middleware) HandlerFunc {
	out := h
	for i := len(chain) - 1; i >= 0; i-- {
		mw := chain[i]
		if mw == nil {
			continue
		}
		out = mw(out)
	}
	return out
}

type fieldErrorer interface {
	FieldErrors() map[string]string
}

func writeJSONError(w http.ResponseWriter, status int, message string, err error) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)

	payload := map[string]any{"message": message}
	if err != nil {
		if fe, ok := err.(fieldErrorer); ok {
			payload["fields"] = fe.FieldErrors()
		}
	}
	_ = json.NewEncoder(w).Encode(payload)
}

func joinPath(prefix, path string) string {
	if path == "" {
		path = "/"
	}
	if !strings.HasPrefix(path, "/") {
		path = "/" + path
	}

	if prefix == "" || prefix == "/" {
		return cleanPath(path)
	}
	if !strings.HasPrefix(prefix, "/") {
		prefix = "/" + prefix
	}
	return cleanPath(strings.TrimRight(prefix, "/") + path)
}

func cleanPath(p string) string {
	if p == "" {
		return "/"
	}
	if !strings.HasPrefix(p, "/") {
		p = "/" + p
	}
	if len(p) > 1 {
		p = strings.TrimRight(p, "/")
	}
	return p
}

func pathSegments(path string) []string {
	if path == "/" {
		return nil
	}
	path = strings.Trim(path, "/")
	if path == "" {
		return nil
	}

	segs := make([]string, 0, 8)
	start := 0
	for i := 0; i < len(path); i++ {
		if path[i] == '/' {
			if start < i {
				segs = append(segs, path[start:i])
			}
			start = i + 1
		}
	}
	if start < len(path) {
		segs = append(segs, path[start:])
	}
	return segs
}

func isParamSegment(seg string) (string, bool) {
	if len(seg) < 3 {
		return "", false
	}
	if seg[0] != '{' || seg[len(seg)-1] != '}' {
		return "", false
	}
	name := seg[1 : len(seg)-1]
	if name == "" || strings.ContainsAny(name, "/{}") {
		return "", false
	}
	return name, true
}
