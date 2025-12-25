package database

import (
	"fmt"
	"reflect"
	"strings"
)

func structToMap(v any) (map[string]any, error) {
	rv := reflect.ValueOf(v)
	if rv.Kind() == reflect.Pointer {
		rv = rv.Elem()
	}
	if rv.Kind() != reflect.Struct {
		return nil, fmt.Errorf("record: expected struct")
	}

	rt := rv.Type()
	out := make(map[string]any, rt.NumField())
	for i := 0; i < rt.NumField(); i++ {
		f := rt.Field(i)
		if f.PkgPath != "" {
			continue
		}
		name, ok := fieldName(f)
		if !ok {
			continue
		}
		out[name] = rv.Field(i).Interface()
	}
	return out, nil
}

func mapToStruct[T any](row map[string]any) (T, error) {
	var out T
	rv := reflect.ValueOf(&out).Elem()
	rt := rv.Type()
	if rt.Kind() != reflect.Struct {
		return out, fmt.Errorf("record: T must be a struct")
	}

	for i := 0; i < rt.NumField(); i++ {
		f := rt.Field(i)
		if f.PkgPath != "" {
			continue
		}
		name, ok := fieldName(f)
		if !ok {
			continue
		}
		val, exists := row[name]
		if !exists {
			continue
		}
		fv := rv.Field(i)
		if !fv.CanSet() {
			continue
		}
		setValue(fv, val)
	}

	return out, nil
}

func fieldName(f reflect.StructField) (string, bool) {
	if tag := f.Tag.Get("db"); tag != "" {
		name := strings.Split(tag, ",")[0]
		name = strings.TrimSpace(name)
		if name == "-" {
			return "", false
		}
		if name != "" {
			return name, true
		}
	}
	if tag := f.Tag.Get("json"); tag != "" {
		name := strings.Split(tag, ",")[0]
		name = strings.TrimSpace(name)
		if name == "-" {
			return "", false
		}
		if name != "" {
			return name, true
		}
	}
	return strings.ToLower(f.Name), true
}

func getID(v any) (any, bool) {
	rv := reflect.ValueOf(v)
	if rv.Kind() == reflect.Pointer {
		rv = rv.Elem()
	}
	if rv.Kind() != reflect.Struct {
		return nil, false
	}
	f := rv.FieldByName("ID")
	if !f.IsValid() {
		f = rv.FieldByName("Id")
	}
	if !f.IsValid() {
		return nil, false
	}
	if isZero(f) {
		return nil, false
	}
	return f.Interface(), true
}

func setID(ptr any, id any) {
	rv := reflect.ValueOf(ptr)
	if rv.Kind() != reflect.Pointer || rv.IsNil() {
		return
	}
	rv = rv.Elem()
	if rv.Kind() != reflect.Struct {
		return
	}
	f := rv.FieldByName("ID")
	if !f.IsValid() {
		f = rv.FieldByName("Id")
	}
	if !f.IsValid() || !f.CanSet() {
		return
	}
	setValue(f, id)
}

func setValue(dst reflect.Value, v any) {
	if v == nil {
		return
	}
	src := reflect.ValueOf(v)
	if src.IsValid() {
		if src.Type().AssignableTo(dst.Type()) {
			dst.Set(src)
			return
		}
		if src.Type().ConvertibleTo(dst.Type()) {
			dst.Set(src.Convert(dst.Type()))
			return
		}
	}

	switch dst.Kind() {
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		switch x := v.(type) {
		case int:
			dst.SetInt(int64(x))
		case int64:
			dst.SetInt(x)
		case float64:
			dst.SetInt(int64(x))
		}
	case reflect.String:
		switch x := v.(type) {
		case string:
			dst.SetString(x)
		}
	}
}

func isZero(v reflect.Value) bool {
	switch v.Kind() {
	case reflect.Invalid:
		return true
	case reflect.String:
		return v.Len() == 0
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return v.Int() == 0
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		return v.Uint() == 0
	case reflect.Bool:
		return !v.Bool()
	default:
		z := reflect.Zero(v.Type())
		return reflect.DeepEqual(v.Interface(), z.Interface())
	}
}
