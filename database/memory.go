package database

import (
	"fmt"
	"sync"
)

type MemoryConnection struct {
	mu     sync.RWMutex
	tables map[string]*memoryTable
}

type memoryTable struct {
	auto  int
	rows  map[any]map[string]any
	order []any
}

func NewMemoryConnection() *MemoryConnection {
	return &MemoryConnection{tables: make(map[string]*memoryTable)}
}

func (m *MemoryConnection) table(name string) *memoryTable {
	t := m.tables[name]
	if t == nil {
		t = &memoryTable{rows: make(map[any]map[string]any)}
		m.tables[name] = t
	}
	return t
}

func (m *MemoryConnection) Find(table string, id any) (map[string]any, bool, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	t := m.tables[table]
	if t == nil {
		return nil, false, nil
	}
	row := t.rows[id]
	if row == nil {
		return nil, false, nil
	}
	return cloneRow(row), true, nil
}

func (m *MemoryConnection) First(table string) (map[string]any, bool, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	t := m.tables[table]
	if t == nil || len(t.order) == 0 {
		return nil, false, nil
	}
	id := t.order[0]
	row := t.rows[id]
	if row == nil {
		return nil, false, nil
	}
	return cloneRow(row), true, nil
}

func (m *MemoryConnection) All(table string) ([]map[string]any, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	t := m.tables[table]
	if t == nil {
		return nil, nil
	}

	out := make([]map[string]any, 0, len(t.order))
	for _, id := range t.order {
		row := t.rows[id]
		if row == nil {
			continue
		}
		out = append(out, cloneRow(row))
	}
	return out, nil
}

func (m *MemoryConnection) Insert(table string, row map[string]any) (any, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	t := m.table(table)

	id := row["id"]
	if id == nil {
		t.auto++
		id = t.auto
		row["id"] = id
	}

	if _, exists := t.rows[id]; exists {
		return nil, fmt.Errorf("memory db: duplicate id")
	}

	t.rows[id] = cloneRow(row)
	t.order = append(t.order, id)
	return id, nil
}

func (m *MemoryConnection) Update(table string, id any, row map[string]any) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	t := m.tables[table]
	if t == nil {
		return fmt.Errorf("memory db: table not found")
	}
	if _, ok := t.rows[id]; !ok {
		return fmt.Errorf("memory db: row not found")
	}

	row["id"] = id
	t.rows[id] = cloneRow(row)
	return nil
}

func (m *MemoryConnection) Delete(table string, id any) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	t := m.tables[table]
	if t == nil {
		return nil
	}
	delete(t.rows, id)
	for i := 0; i < len(t.order); i++ {
		if t.order[i] == id {
			copy(t.order[i:], t.order[i+1:])
			t.order = t.order[:len(t.order)-1]
			break
		}
	}
	return nil
}

func cloneRow(in map[string]any) map[string]any {
	out := make(map[string]any, len(in))
	for k, v := range in {
		out[k] = v
	}
	return out
}
