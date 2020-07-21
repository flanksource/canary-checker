package utils

import (
	"reflect"
	"strings"
)

type StructTemplater struct {
	Values map[string]string
}

// this func is required to fulfil the reflectwalk.StructWalker interface
func (w StructTemplater) Struct(reflect.Value) error {
	return nil
}

func (w StructTemplater) StructField(f reflect.StructField, v reflect.Value) error {
	if v.CanSet() && v.Kind() == reflect.String {
		v.SetString(w.Template(v.String()))
	}
	return nil
}

func (w StructTemplater) Template(val string) string {
	if strings.HasPrefix(val, "$") {
		key := strings.TrimRight(strings.TrimLeft(val[1:], "("), ")")
		env := w.Values[key]
		if env != "" {
			return env
		}
	}
	return val
}
