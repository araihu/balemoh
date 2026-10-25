package manifest

import (
	"encoding/json"
	"errors"
	"sort"
)

// Field is the deliberately small, host-editable configuration vocabulary.
type Field struct {
	Name        string          `json:"-"`
	Type        string          `json:"type"`
	Title       string          `json:"title"`
	Description string          `json:"description"`
	Enum        []string        `json:"enum"`
	MaxLength   int             `json:"maxLength"`
	Minimum     *float64        `json:"minimum"`
	Maximum     *float64        `json:"maximum"`
	Default     json.RawMessage `json:"default"`
	Items       *Field          `json:"items"`
	Required    bool            `json:"-"`
}

func FormFields(raw []byte) ([]Field, error) {
	var schema struct {
		Type       string                     `json:"type"`
		Properties map[string]json.RawMessage `json:"properties"`
		Required   []string                   `json:"required"`
		Additional *bool                      `json:"additionalProperties"`
	}
	if json.Unmarshal(raw, &schema) != nil || schema.Type != "object" || schema.Additional == nil || *schema.Additional || len(schema.Properties) > 32 {
		return nil, errors.New("unsupported form schema")
	}
	required := map[string]bool{}
	for _, k := range schema.Required {
		required[k] = true
	}
	fields := []Field{}
	for name, raw := range schema.Properties {
		if len(name) > 64 || name == "" {
			return nil, errors.New("invalid form field")
		}
		for _, c := range name {
			if c != '-' && c != '_' && (c < 'a' || c > 'z') && (c < 'A' || c > 'Z') && (c < '0' || c > '9') {
				return nil, errors.New("invalid form field")
			}
		}
		var f Field
		var properties map[string]any
		_ = json.Unmarshal(raw, &properties)
		if json.Unmarshal(raw, &f) != nil {
			return nil, errors.New("invalid form field")
		}
		for key := range properties {
			switch key {
			case "type", "title", "description", "enum", "maxLength", "minLength", "minimum", "maximum", "default", "items", "maxItems", "minItems", "uniqueItems":
			default:
				return nil, errors.New("unsupported form validation")
			}
		}
		switch f.Type {
		case "string":
			if f.MaxLength <= 0 && len(f.Enum) == 0 {
				return nil, errors.New("form string needs bound")
			}
		case "boolean":
		case "integer", "number":
			if f.Minimum == nil || f.Maximum == nil {
				return nil, errors.New("form number needs bounds")
			}
		case "array":
			if f.Items == nil || f.Items.Type != "string" || len(f.Items.Enum) == 0 {
				return nil, errors.New("form array needs string enum")
			}
		default:
			return nil, errors.New("unsupported form type")
		}
		f.Name = name
		f.Required = required[name]
		if f.Title == "" {
			f.Title = name
		}
		fields = append(fields, f)
	}
	sort.Slice(fields, func(i, j int) bool { return fields[i].Name < fields[j].Name })
	return fields, nil
}
