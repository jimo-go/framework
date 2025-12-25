package validation

import (
	"fmt"
	"net/mail"
	"reflect"
	"strconv"
	"strings"
)

type Error struct {
	Fields map[string]string
}

func (e Error) Error() string {
	return "validation failed"
}

func (e Error) FieldErrors() map[string]string {
	return e.Fields
}

type Rules map[string]string

func Validate(v any, rules Rules) (Error, bool) {
	fields := make(map[string]string)

	rv := reflect.ValueOf(v)
	if rv.Kind() == reflect.Pointer {
		rv = rv.Elem()
	}
	if rv.Kind() != reflect.Struct {
		return Error{Fields: map[string]string{"_": "Invalid payload"}}, true
	}

	rt := rv.Type()
	for i := 0; i < rt.NumField(); i++ {
		f := rt.Field(i)
		if f.PkgPath != "" {
			continue
		}

		name := fieldName(f)
		ruleStr, ok := rules[name]
		if !ok {
			continue
		}

		msg := applyRule(rv.Field(i), name, ruleStr)
		if msg != "" {
			fields[name] = msg
		}
	}

	if len(fields) == 0 {
		return Error{}, false
	}
	return Error{Fields: fields}, true
}

func fieldName(f reflect.StructField) string {
	if tag := f.Tag.Get("json"); tag != "" {
		name := strings.Split(tag, ",")[0]
		name = strings.TrimSpace(name)
		if name != "" && name != "-" {
			return name
		}
	}
	return strings.ToLower(f.Name)
}

func applyRule(v reflect.Value, name string, ruleStr string) string {
	parts := strings.Split(ruleStr, "|")
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}

		key, arg, _ := strings.Cut(p, ":")
		switch key {
		case "required":
			if isEmpty(v) {
				return fmt.Sprintf("%s is required", name)
			}
		case "email":
			s := asString(v)
			if s == "" {
				continue
			}
			if _, err := mail.ParseAddress(s); err != nil {
				return fmt.Sprintf("%s must be a valid email", name)
			}
		case "min":
			n, _ := strconv.Atoi(arg)
			if n > 0 {
				if len(asString(v)) < n {
					return fmt.Sprintf("%s must be at least %d characters", name, n)
				}
			}
		case "max":
			n, _ := strconv.Atoi(arg)
			if n > 0 {
				if len(asString(v)) > n {
					return fmt.Sprintf("%s must be at most %d characters", name, n)
				}
			}
		}
	}
	return ""
}

func asString(v reflect.Value) string {
	if !v.IsValid() {
		return ""
	}
	if v.Kind() == reflect.Pointer {
		if v.IsNil() {
			return ""
		}
		v = v.Elem()
	}
	switch v.Kind() {
	case reflect.String:
		return v.String()
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return strconv.FormatInt(v.Int(), 10)
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		return strconv.FormatUint(v.Uint(), 10)
	default:
		return ""
	}
}

func isEmpty(v reflect.Value) bool {
	if !v.IsValid() {
		return true
	}
	if v.Kind() == reflect.Pointer {
		return v.IsNil()
	}
	switch v.Kind() {
	case reflect.String:
		return strings.TrimSpace(v.String()) == ""
	case reflect.Bool:
		return !v.Bool()
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return v.Int() == 0
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		return v.Uint() == 0
	default:
		z := reflect.Zero(v.Type())
		return reflect.DeepEqual(v.Interface(), z.Interface())
	}
}
