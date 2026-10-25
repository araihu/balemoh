package view

import (
	"bytes"
	"encoding/json"
	"fmt"
	"github.com/araihu/balemoh/plugin-sdk/manifest"
	"strings"
)

type PluginForm struct {
	ID, Revision, Action, Title, Error string
	Fields                             []manifest.Field
	Values                             map[string]any
	Enabled                            bool
	Create                             bool
}
type PluginManagement struct {
	Name, PluginName, ConnectionName, Error, ReloadURL string
	Form                                               PluginForm
	OptionsAvailable                                   bool
}
type PluginWidgetRow struct {
	ID, Title, Plugin, Connection, URL string
	Enabled                            bool
}
type PluginWidgetList struct {
	Saved, Available []PluginWidgetRow
}

func (f PluginForm) Value(key string) string {
	v := f.Values[key]
	if v == nil {
		return ""
	}
	return fmt.Sprint(v)
}
func (f PluginForm) Checked(key, value string) bool {
	v := f.Values[key]
	if value == "true" {
		return v == true
	}
	if xs, ok := v.([]any); ok {
		for _, x := range xs {
			if x == value {
				return true
			}
		}
	}
	return false
}
func PluginValues(raw []byte) map[string]any {
	var v map[string]any
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.UseNumber()
	_ = decoder.Decode(&v)
	if v == nil {
		v = map[string]any{}
	}
	return v
}

func nativePluginURL(raw string) string {
	sep := "?"
	if strings.Contains(raw, "?") {
		sep = "&"
	}
	return raw + sep + "__native=1"
}

func fieldHints(description string) []string {
	if description == "" {
		return nil
	}
	return []string{description}
}
