package http

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"

	"github.com/jimo-go/framework/validation"
)

// Context wraps http.ResponseWriter and *http.Request and provides ergonomic helpers.
type Context struct {
	ResponseWriter http.ResponseWriter
	Request        *http.Request

	params map[string]string
	views  *viewEngine

	session *Session
	csrf    string
}

// HTTPError is a typed error used to propagate HTTP failures through panics.
//
// Router recovers these errors and converts them into responses.
type HTTPError struct {
	Status  int
	Message string
	Err     error
}

func (e HTTPError) Error() string {
	if e.Err == nil {
		return e.Message
	}
	return e.Message + ": " + e.Err.Error()
}

func (e HTTPError) Unwrap() error { return e.Err }

// NewContext creates a new request context.
func NewContext(w http.ResponseWriter, r *http.Request, views *viewEngine) *Context {
	return &Context{ResponseWriter: w, Request: r, views: views}
}

// Param returns a route parameter by name.
func (c *Context) Param(name string) string {
	if c.params == nil {
		return ""
	}
	return c.params[name]
}

// Session returns the current request session.
//
// It is nil unless the Sessions middleware is enabled.
func (c *Context) Session() *Session {
	return c.session
}

// CSRFToken returns the CSRF token for the current session.
//
// It is empty unless Sessions+CSRF middleware is enabled.
func (c *Context) CSRFToken() string {
	return c.csrf
}

// MustValidate validates a struct against a set of rules.
//
// On failure, it panics with an HTTPError (422) and attaches the validation error as Err.
func (c *Context) MustValidate(v any, rules validation.Rules) {
	err, failed := validation.Validate(v, rules)
	if !failed {
		return
	}
	panic(HTTPError{Status: http.StatusUnprocessableEntity, Message: "Validation failed", Err: err})
}

// JSON writes a JSON response.
func (c *Context) JSON(status int, data any) {
	c.ResponseWriter.Header().Set("Content-Type", "application/json; charset=utf-8")
	c.ResponseWriter.WriteHeader(status)
	if err := json.NewEncoder(c.ResponseWriter).Encode(data); err != nil {
		panic(HTTPError{Status: http.StatusInternalServerError, Message: "Failed to encode JSON", Err: err})
	}
}

// String writes a plain-text response.
func (c *Context) String(status int, text string) {
	c.ResponseWriter.Header().Set("Content-Type", "text/plain; charset=utf-8")
	c.ResponseWriter.WriteHeader(status)
	_, _ = io.WriteString(c.ResponseWriter, text)
}

// View renders an HTML template from the configured views directory.
//
// On failure, it panics with an HTTPError (500).
func (c *Context) View(name string, data any) {
	if c.views == nil {
		panic(HTTPError{Status: http.StatusInternalServerError, Message: "View engine is not configured"})
	}

	c.ResponseWriter.Header().Set("Content-Type", "text/html; charset=utf-8")
	c.ResponseWriter.WriteHeader(http.StatusOK)

	if err := c.views.Render(c.ResponseWriter, name, data); err != nil {
		panic(HTTPError{Status: http.StatusInternalServerError, Message: "Failed to render view", Err: err})
	}
}

// MustBind decodes the JSON request body into v and validates that:
// - JSON is syntactically valid
// - Unknown fields are rejected
// - The body contains exactly one JSON value
//
// On failure, it panics with an HTTPError (400).
func (c *Context) MustBind(v any) {
	if v == nil {
		panic(HTTPError{Status: http.StatusBadRequest, Message: "Invalid JSON", Err: errors.New("bind target is nil")})
	}

	dec := json.NewDecoder(c.Request.Body)
	dec.DisallowUnknownFields()

	if err := dec.Decode(v); err != nil {
		panic(HTTPError{Status: http.StatusBadRequest, Message: "Invalid JSON", Err: err})
	}

	// Ensure there is no trailing JSON.
	if err := dec.Decode(&struct{}{}); err != io.EOF {
		if err == nil {
			err = errors.New("unexpected trailing JSON")
		}
		panic(HTTPError{Status: http.StatusBadRequest, Message: "Invalid JSON", Err: err})
	}
}
