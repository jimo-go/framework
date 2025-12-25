package http

import (
	"net/http"
	"strings"
)

// Sessions loads and saves a cookie-backed session for each request.
func Sessions(sm *SessionManager) Middleware {
	return func(next HandlerFunc) HandlerFunc {
		return func(ctx *Context) {
			if sm == nil {
				next(ctx)
				return
			}
			s := sm.load(ctx.Request)
			ctx.session = s
			defer func() {
				_ = sm.save(ctx.ResponseWriter, s)
			}()
			next(ctx)
		}
	}
}

// CSRF protects unsafe methods using the token stored in session.
//
// It expects the token in one of:
// - Header: X-CSRF-Token
// - Form: _token
func CSRF(sm *SessionManager) Middleware {
	return func(next HandlerFunc) HandlerFunc {
		return func(ctx *Context) {
			if sm == nil || ctx.session == nil {
				next(ctx)
				return
			}

			ctx.csrf = ctx.session.CSRF

			// API JSON endpoints are typically stateless and don't use CSRF cookies.
			// We skip CSRF checks for /api/* and JSON requests.
			if strings.HasPrefix(ctx.Request.URL.Path, "/api/") || strings.HasPrefix(ctx.Request.Header.Get("Content-Type"), "application/json") {
				next(ctx)
				return
			}

			switch ctx.Request.Method {
			case http.MethodGet, http.MethodHead, http.MethodOptions:
				next(ctx)
				return
			}

			token := ctx.Request.Header.Get("X-CSRF-Token")
			if token == "" {
				token = ctx.Request.FormValue("_token")
			}
			if token == "" || token != ctx.session.CSRF {
				panic(HTTPError{Status: 419, Message: "CSRF token mismatch"})
			}
			next(ctx)
		}
	}
}
