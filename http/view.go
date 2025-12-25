package http

import (
	"fmt"
	"html/template"
	"io"
	"path/filepath"
	"strings"
	"sync"
)

type viewEngine struct {
	dir   string
	mu    sync.RWMutex
	cache map[string]*template.Template
}

func newViewEngine(dir string) *viewEngine {
	return &viewEngine{dir: dir, cache: make(map[string]*template.Template)}
}

func (v *viewEngine) SetDir(dir string) {
	v.mu.Lock()
	defer v.mu.Unlock()
	v.dir = dir
	v.cache = make(map[string]*template.Template)
}

func (v *viewEngine) Render(w io.Writer, name string, data any) error {
	tpl, err := v.template(name)
	if err != nil {
		return err
	}
	return tpl.Execute(w, data)
}

func (v *viewEngine) template(name string) (*template.Template, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return nil, fmt.Errorf("view: template name is empty")
	}
	if strings.Contains(name, "..") {
		return nil, fmt.Errorf("view: invalid template name")
	}
	if filepath.Ext(name) == "" {
		name += ".html"
	}

	v.mu.RLock()
	tpl := v.cache[name]
	dir := v.dir
	v.mu.RUnlock()
	if tpl != nil {
		return tpl, nil
	}

	path := filepath.Join(dir, name)
	parsed, err := template.ParseFiles(path)
	if err != nil {
		return nil, err
	}

	v.mu.Lock()
	defer v.mu.Unlock()
	if existing := v.cache[name]; existing != nil {
		return existing, nil
	}
	v.cache[name] = parsed
	return parsed, nil
}
