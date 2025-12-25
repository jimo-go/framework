package core

import (
	"fmt"
	"reflect"
	"sync"
)

// Provider is a factory function used by the container to construct a service.
//
// Providers may call Resolve to fetch other dependencies.
type Provider func(*Container) (any, error)

// Container is a thread-safe service container.
//
// It is intentionally small and opinionated: services are registered by their Go type.
// This enables an ergonomic, compile-time-friendly dependency injection style using generics.
type Container struct {
	mu        sync.RWMutex
	providers map[reflect.Type]Provider
}

// NewContainer creates a new, empty service container.
func NewContainer() *Container {
	return &Container{
		providers: make(map[reflect.Type]Provider),
	}
}

func typeKey[T any]() reflect.Type {
	var ptr *T
	return reflect.TypeOf(ptr).Elem()
}

// Bind registers a provider for the given service type.
//
// If the type is already bound, Bind returns an error.
func (c *Container) Bind(t reflect.Type, provider Provider) error {
	if t == nil {
		return fmt.Errorf("container: type is nil")
	}
	if provider == nil {
		return fmt.Errorf("container: provider is nil")
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	if _, exists := c.providers[t]; exists {
		return fmt.Errorf("container: provider already bound for %s", t.String())
	}

	c.providers[t] = provider
	return nil
}

// Resolve constructs and returns a service instance for the given type.
func (c *Container) Resolve(t reflect.Type) (any, error) {
	if t == nil {
		return nil, fmt.Errorf("container: type is nil")
	}

	c.mu.RLock()
	provider, ok := c.providers[t]
	c.mu.RUnlock()

	if !ok {
		return nil, fmt.Errorf("container: no provider bound for %s", t.String())
	}

	return provider(c)
}

// MustResolve is like Resolve but panics on error.
func (c *Container) MustResolve(t reflect.Type) any {
	v, err := c.Resolve(t)
	if err != nil {
		panic(err)
	}
	return v
}

// Bind registers a provider for type T.
//
// This is a package-level helper because Go does not support generic methods.
func Bind[T any](c *Container, provider func(*Container) (T, error)) error {
	if c == nil {
		return fmt.Errorf("container: container is nil")
	}
	if provider == nil {
		return fmt.Errorf("container: provider is nil")
	}

	key := typeKey[T]()
	return c.Bind(key, func(c *Container) (any, error) {
		return provider(c)
	})
}

// Resolve returns an instance of type T by calling the registered provider.
//
// This is a package-level helper because Go does not support generic methods.
func Resolve[T any](c *Container) (T, error) {
	if c == nil {
		var zero T
		return zero, fmt.Errorf("container: container is nil")
	}

	key := typeKey[T]()
	v, err := c.Resolve(key)
	if err != nil {
		var zero T
		return zero, err
	}

	service, ok := v.(T)
	if !ok {
		var zero T
		return zero, fmt.Errorf("container: provider returned %T, expected %s", v, key.String())
	}
	return service, nil
}

// MustResolve is like Resolve but panics on error.
func MustResolve[T any](c *Container) T {
	v, err := Resolve[T](c)
	if err != nil {
		panic(err)
	}
	return v
}
