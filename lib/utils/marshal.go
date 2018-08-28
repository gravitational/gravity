package utils

import (
	"encoding/json"
	"io"

	"github.com/ghodss/yaml"
	"github.com/gravitational/trace"
)

// WriteJSON writes JSON serialized object into provided writer
func WriteJSON(m Marshaler, w io.Writer) error {
	data, err := json.MarshalIndent(m.ToMarshal(), "", "    ")
	if err != nil {
		return trace.Wrap(err)
	}
	_, err = w.Write(data)
	return trace.Wrap(err)
}

// WriteYAML writes YAML serialized object into provided writer
func WriteYAML(m Marshaler, w io.Writer) error {
	data, err := yaml.Marshal(m.ToMarshal())
	if err != nil {
		return trace.Wrap(err)
	}
	_, err = w.Write(data)
	return trace.Wrap(err)
}

// Marshaler defines an interface for marshalable objects
type Marshaler interface {
	// ToMarshal returns object that needs to be marshaled
	ToMarshal() interface{}
}
