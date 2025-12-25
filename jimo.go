package jimo

import (
	"github.com/jimo-go/framework/core"
	jimohttp "github.com/jimo-go/framework/http"
)

// App is the main application type.
//
// It is a facade over the internal kernel implementation to provide a clean public API.
// The goal is great DX: most users should only import the top-level framework package.
type App = core.Jimo

// Config is the application configuration.
type Config = core.Config

// Context is the request context passed into handlers.
type Context = jimohttp.Context

// HandlerFunc is the primary handler signature for JIMO.
type HandlerFunc = jimohttp.HandlerFunc

// Middleware is a router middleware.
type Middleware = jimohttp.Middleware

// RouteOption configures per-route behavior.
type RouteOption = jimohttp.RouteOption

// New creates a new Jimo application instance.
func New() *App {
	return core.New()
}

// Named assigns a name to a route.
func Named(name string) RouteOption { return jimohttp.Named(name) }

// WithMiddleware attaches middleware to a single route.
func WithMiddleware(mw ...Middleware) RouteOption { return jimohttp.WithMiddleware(mw...) }
