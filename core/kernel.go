package core

import (
	"net/http"
	"os"
	"time"

	jimohttp "github.com/jimo-go/framework/http"
)

// Jimo is the framework kernel and the primary entry point of the application.
//
// It aggregates the service container, router, and server configuration.
type Jimo struct {
	Container *Container
	Router    *jimohttp.Router
	Config    *Config

	// Server is optional. If nil, Listen will create a default http.Server.
	Server *http.Server
}

// New creates a new Jimo application instance with a default container and router.
func New() *Jimo {
	_ = AutoLoadEnv(".")
	cfg := NewConfig()
	if cfg.Key == "" {
		if k, err := GenerateAppKey(); err == nil {
			cfg.Key = k
		}
	}
	return &Jimo{
		Container: NewContainer(),
		Router:    jimohttp.NewRouter(),
		Config:    cfg,
	}
}

// LoadEnv loads a dotenv file into the process environment (non-overwriting) and refreshes app config.
func (j *Jimo) LoadEnv(path string) error {
	if err := LoadEnv(path); err != nil {
		return err
	}
	if j.Config == nil {
		j.Config = NewConfig()
		return nil
	}
	j.Config.RefreshFromEnv()
	return nil
}

// AutoLoadEnv loads .env from a directory if present and refreshes app config.
func (j *Jimo) AutoLoadEnv(dir string) error {
	if err := AutoLoadEnv(dir); err != nil {
		return err
	}
	if j.Config == nil {
		j.Config = NewConfig()
		return nil
	}
	j.Config.RefreshFromEnv()
	return nil
}

// Env returns current application environment name.
func (j *Jimo) Env() string {
	if j.Config == nil {
		return ""
	}
	return j.Config.Env
}

// Debug indicates whether the application is running in debug mode.
func (j *Jimo) Debug() bool {
	if j.Config == nil {
		return false
	}
	return j.Config.Debug
}

// Root returns the process working directory.
func (j *Jimo) Root() string {
	wd, _ := os.Getwd()
	return wd
}

// Web enables the default "web" middleware stack: sessions + CSRF.
func (j *Jimo) Web() error {
	if j.Config == nil {
		j.Config = NewConfig()
	}

	sm, err := jimohttp.NewSessionManager(j.Config.Key)
	if err != nil {
		return err
	}

	j.Use(
		jimohttp.Sessions(sm),
		jimohttp.CSRF(sm),
	)

	return nil
}

// MustWeb is like Web but panics on error.
func (j *Jimo) MustWeb() {
	if err := j.Web(); err != nil {
		panic(err)
	}
}

// Get registers a GET route.
func (j *Jimo) Get(path string, handler jimohttp.HandlerFunc, opts ...jimohttp.RouteOption) {
	j.Router.Get(path, handler, opts...)
}

// Post registers a POST route.
func (j *Jimo) Post(path string, handler jimohttp.HandlerFunc, opts ...jimohttp.RouteOption) {
	j.Router.Post(path, handler, opts...)
}

// Group registers a group of routes under a common prefix.
func (j *Jimo) Group(prefix string, fn func(r *jimohttp.Router)) {
	j.Router.Group(prefix, fn)
}

// Use registers middleware globally for the application.
func (j *Jimo) Use(mw ...jimohttp.Middleware) {
	j.Router.Use(mw...)
}

// URL returns a route path by its name.
func (j *Jimo) URL(name string, params map[string]string) string {
	return j.Router.URL(name, params)
}

// Views configures the directory used for rendering templates via Context.View().
func (j *Jimo) Views(dir string) {
	j.Router.SetViewsDir(dir)
}

// Listen starts the HTTP server on the given address.
func (j *Jimo) Listen(addr string) error {
	srv := j.Server
	if srv == nil {
		srv = &http.Server{
			Addr:              addr,
			Handler:           j.Router,
			ReadHeaderTimeout: 5 * time.Second,
		}
	}

	if srv.Addr == "" {
		srv.Addr = addr
	}
	if srv.Handler == nil {
		srv.Handler = j.Router
	}

	return srv.ListenAndServe()
}
