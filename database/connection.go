package database

import "sync"

// Connection is the minimal persistence contract used by the Active Record layer.
//
// It is intentionally small for Phase 1; SQL backends can be implemented later.
type Connection interface {
	Find(table string, id any) (row map[string]any, ok bool, err error)
	First(table string) (row map[string]any, ok bool, err error)
	All(table string) ([]map[string]any, error)
	Insert(table string, row map[string]any) (id any, err error)
	Update(table string, id any, row map[string]any) error
	Delete(table string, id any) error
}

var (
	defaultConn Connection
	defaultMu   sync.RWMutex
)

// Use sets the default connection used by Model[T]().
func Use(conn Connection) {
	defaultMu.Lock()
	defer defaultMu.Unlock()
	defaultConn = conn
}

// Default returns the currently configured default connection.
func Default() Connection {
	defaultMu.RLock()
	defer defaultMu.RUnlock()
	return defaultConn
}
