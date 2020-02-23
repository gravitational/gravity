/*
Copyright 2018 Gravitational, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package defaults

import (
	"encoding/json"
	"reflect"
	"strings"

	"github.com/gravitational/trace"
	"github.com/santhosh-tekuri/jsonschema"
)

// Apply applies defaults from schema to the given object v
// which is expected to conform to said schema
func Apply(v interface{}, schema *jsonschema.Schema) error {
	if schema == nil {
		return nil
	}

	valueType := reflect.TypeOf(v)
	value := reflect.ValueOf(v)

	if valueType.Kind() != reflect.Ptr || reflect.Indirect(value).Kind() != reflect.Struct {
		return trace.BadParameter("expected a reference to a struct, but got %T", v)
	}

	valueType = valueType.Elem()
	value = value.Elem()

	r := reflector{TagName: "json"}
	return r.reflectFromType(valueType, value, schema)
}

func (r reflector) reflectFromType(t reflect.Type, v reflect.Value, schema *jsonschema.Schema) error {
	if schema.Ref != nil {
		schema = schema.Ref
	}
	switch t.Kind() {
	case reflect.Struct:
		return r.reflectStruct(t, v, schema)

	case reflect.Map:
		for _, key := range v.MapKeys() {
			elem := v.MapIndex(key)
			if err := r.reflectFromType(elem.Type(), elem, schema); err != nil {
				return trace.Wrap(err)
			}
		}

	case reflect.Slice, reflect.Array:
		switch itemsSchema := schema.Items.(type) {
		case *jsonschema.Schema:
			for i := 0; i < v.Len(); i++ {
				elem := v.Index(i)
				if err := r.reflectFromType(elem.Type(), elem, itemsSchema); err != nil {
					return trace.Wrap(err)
				}
			}
		}

	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64,
		reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		if schema.Default != nil {
			switch defaultValue := schema.Default.(type) {
			case json.Number:
				numeric, err := defaultValue.Int64()
				if err != nil {
					return trace.Wrap(err)
				}
				setPrimitiveValue(t, v, numeric)
			default:
				setPrimitiveValue(t, v, schema.Default)
			}
		}
	case reflect.Float32, reflect.Float64:
		if schema.Default != nil {
			switch defaultValue := schema.Default.(type) {
			case json.Number:
				numeric, err := defaultValue.Float64()
				if err != nil {
					return trace.Wrap(err)
				}
				setPrimitiveValue(t, v, numeric)
			default:
				setPrimitiveValue(t, v, schema.Default)
			}
		}
	case reflect.Bool, reflect.String:
		if schema.Default != nil {
			setPrimitiveValue(t, v, schema.Default)
		}

	case reflect.Ptr:
		elemType := t.Elem()
		if !v.IsNil() &&
			isPrimitive(elemType) && elemType.Kind() != reflect.String &&
			schema.Default != nil {
			// If it's an pointer to a primitive value (except string) and the default
			// has been specified, consider this value already initialized
			return nil
		}

		if v.IsNil() && schema.Default != nil {
			v.Set(reflect.New(elemType))
		}
		return r.reflectFromType(elemType, v.Elem(), schema)
	}

	return nil
}

func (r reflector) reflectStruct(t reflect.Type, v reflect.Value, schema *jsonschema.Schema) error {
	if schema.Ref != nil {
		schema = schema.Ref
	}
	if t.Kind() == reflect.Ptr {
		t = t.Elem()
		v = v.Elem()
	}
	for i := 0; i < t.NumField(); i++ {
		field := t.Field(i)

		if !shouldReflectField(field) {
			continue
		}

		v = reflect.Indirect(v)
		if !v.IsValid() {
			continue
		}
		value := v.Field(i)
		fieldName, _ := r.tag(field)
		if field.Anonymous {
			fieldSchema := schema
			if fieldName != "" {
				fieldSchema = schema.Properties[fieldName]
			}
			if fieldSchema != nil {
				if err := r.reflectFromType(field.Type, value, fieldSchema); err != nil {
					return trace.Wrap(err)
				}
			}
			continue
		}

		if fieldSchema := schema.Properties[fieldName]; fieldSchema != nil {
			if err := r.reflectFromType(field.Type, value, fieldSchema); err != nil {
				return trace.Wrap(err)
			}
		}
	}
	return nil
}

func (r reflector) tag(field reflect.StructField) (name string, options []string) {
	spec := strings.Split(field.Tag.Get(r.TagName), ",")
	name, options = spec[0], spec[1:]
	return name, options
}

func setPrimitiveValue(receiverType reflect.Type, receiver reflect.Value, value interface{}) {
	if isEmptyValue(receiver.Interface()) && receiver.CanSet() {
		receiver.Set(reflect.ValueOf(value).Convert(receiverType))
	}
}

func isPrimitive(t reflect.Type) bool {
	switch t.Kind() {
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64,
		reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64,
		reflect.Float32, reflect.Float64,
		reflect.Bool, reflect.String:
		return true
	}
	return false
}

func isEmptyValue(v interface{}) bool {
	switch typ := v.(type) {
	case []interface{}:
		return len(typ) == 0
	case []string:
		return len(typ) == 0
	case map[string]interface{}:
		return len(typ) == 0
	case bool:
		return !typ
	case float64:
		return typ == 0
	case string:
		return typ == ""
	case nil:
		return true
	default:
		value := reflect.ValueOf(v)
		return value.Interface() == reflect.Zero(value.Type()).Interface()
	}
}

func shouldReflectField(field reflect.StructField) bool {
	//nolint:gosimple
	if field.PkgPath != "" {
		// Skip unexported fields
		return false
	}
	return true
}

type reflector struct {
	// TagName defines the tag to use for reflection.
	// Reflector uses the struct tags to map field name
	// to the attribute name as specified in the schema.
	TagName string
}
