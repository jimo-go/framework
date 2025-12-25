package database

import (
	"fmt"
	"reflect"
	"strings"
)

type TableNamer interface {
	TableName() string
}

type Record[T any] struct {
	conn  Connection
	table string
	pk    string
}

func Model[T any]() *Record[T] {
	var zero T

	conn := Default()
	if conn == nil {
		conn = NewMemoryConnection()
		Use(conn)
	}

	table := defaultTableName(zero)
	return &Record[T]{conn: conn, table: table, pk: "id"}
}

func (r *Record[T]) Table(name string) *Record[T] {
	if name != "" {
		r.table = name
	}
	return r
}

func (r *Record[T]) Find(id any) (T, bool, error) {
	row, ok, err := r.conn.Find(r.table, id)
	if err != nil {
		var zero T
		return zero, false, err
	}
	if !ok {
		var zero T
		return zero, false, nil
	}
	v, err := mapToStruct[T](row)
	if err != nil {
		var zero T
		return zero, false, err
	}
	return v, true, nil
}

func (r *Record[T]) FindFirst() (T, bool, error) {
	row, ok, err := r.conn.First(r.table)
	if err != nil {
		var zero T
		return zero, false, err
	}
	if !ok {
		var zero T
		return zero, false, nil
	}
	v, err := mapToStruct[T](row)
	if err != nil {
		var zero T
		return zero, false, err
	}
	return v, true, nil
}

func (r *Record[T]) All() ([]T, error) {
	rows, err := r.conn.All(r.table)
	if err != nil {
		return nil, err
	}
	out := make([]T, 0, len(rows))
	for _, row := range rows {
		v, err := mapToStruct[T](row)
		if err != nil {
			return nil, err
		}
		out = append(out, v)
	}
	return out, nil
}

func (r *Record[T]) Create(v *T) error {
	if v == nil {
		return fmt.Errorf("record: value is nil")
	}

	row, err := structToMap(*v)
	if err != nil {
		return err
	}
	id, err := r.conn.Insert(r.table, row)
	if err != nil {
		return err
	}
	setID(v, id)
	return nil
}

func (r *Record[T]) Save(v *T) error {
	if v == nil {
		return fmt.Errorf("record: value is nil")
	}

	id, ok := getID(*v)
	if !ok {
		return fmt.Errorf("record: missing id")
	}
	row, err := structToMap(*v)
	if err != nil {
		return err
	}
	return r.conn.Update(r.table, id, row)
}

func (r *Record[T]) Delete(id any) error {
	return r.conn.Delete(r.table, id)
}

func defaultTableName[T any](v T) string {
	if tn, ok := any(v).(TableNamer); ok {
		if name := strings.TrimSpace(tn.TableName()); name != "" {
			return name
		}
	}
	// fallback: type name -> snake-ish + plural 's'
	t := reflect.TypeOf(v)
	if t.Kind() == reflect.Pointer {
		t = t.Elem()
	}
	name := t.Name()
	if name == "" {
		return "records"
	}
	return strings.ToLower(name) + "s"
}
