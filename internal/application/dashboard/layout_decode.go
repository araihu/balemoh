package dashboard

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"reflect"
	"strings"
	"unicode/utf8"
)

// DecodeLayoutJSON enforces presence, types, non-null fields and exact object shapes.
func DecodeLayoutJSON(raw []byte, target any) error {
	invalid := func() error {
		return LayoutFailure(400, "invalid_request", "Expected one complete layout JSON value with no unknown, repeated or null fields.")
	}
	if !utf8.Valid(raw) {
		return invalid()
	}
	d := json.NewDecoder(bytes.NewReader(raw))
	d.UseNumber()
	if err := uniqueJSON(d); err != nil {
		return invalid()
	}
	if _, e := d.Token(); e != io.EOF {
		return invalid()
	}
	if e := layoutShape(raw, reflect.TypeOf(target).Elem()); e != nil {
		return invalid()
	}
	d = json.NewDecoder(bytes.NewReader(raw))
	d.DisallowUnknownFields()
	if e := d.Decode(target); e != nil {
		return invalid()
	}
	return nil
}
func uniqueJSON(d *json.Decoder) error {
	t, e := d.Token()
	if e != nil {
		return e
	}
	delim, ok := t.(json.Delim)
	if !ok {
		return nil
	}
	switch delim {
	case '{':
		seen := map[string]bool{}
		for d.More() {
			k, e := d.Token()
			if e != nil {
				return e
			}
			key, ok := k.(string)
			if !ok || seen[key] {
				return fmt.Errorf("duplicate key")
			}
			seen[key] = true
			if e := uniqueJSON(d); e != nil {
				return e
			}
		}
	case '[':
		for d.More() {
			if e := uniqueJSON(d); e != nil {
				return e
			}
		}
	default:
		return fmt.Errorf("unexpected delimiter")
	}
	_, e = d.Token()
	return e
}
func layoutShape(raw json.RawMessage, t reflect.Type) error {
	if t == reflect.TypeFor[json.RawMessage]() {
		return nil
	}
	if bytes.Equal(bytes.TrimSpace(raw), []byte("null")) {
		return fmt.Errorf("null")
	}
	switch t.Kind() {
	case reflect.Struct:
		var obj map[string]json.RawMessage
		if e := json.Unmarshal(raw, &obj); e != nil {
			return e
		}
		known := map[string]bool{}
		for i := 0; i < t.NumField(); i++ {
			f := t.Field(i)
			tag := f.Tag.Get("json")
			name, opts, _ := strings.Cut(tag, ",")
			if name == "-" {
				continue
			}
			known[name] = true
			v, ok := obj[name]
			if !ok {
				if strings.Contains(opts, "omitempty") {
					continue
				}
				return fmt.Errorf("missing %s", name)
			}
			if e := layoutShape(v, f.Type); e != nil {
				return e
			}
		}
		for k := range obj {
			if !known[k] {
				return fmt.Errorf("unknown %s", k)
			}
		}
	case reflect.Slice:
		var a []json.RawMessage
		if e := json.Unmarshal(raw, &a); e != nil {
			return e
		}
		for _, v := range a {
			if e := layoutShape(v, t.Elem()); e != nil {
				return e
			}
		}
	}
	return nil
}
