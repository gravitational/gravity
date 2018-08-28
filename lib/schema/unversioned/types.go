package unversioned

// MultiSourceValue defines a type with multiple value sources
type MultiSourceValue struct {
	// Env is the name of environment variable to read value from
	Env string `json:"env,omitempty"`
	// Path is the path to the file to read value from
	Path string `json:"path,omitempty"`
	// Value is the literal value
	Value string `json:"value,omitempty"`
}

// IsEmpty determines if this multi-source value is empty
func (v MultiSourceValue) IsEmpty() bool {
	return v.Env == "" && v.Path == "" && v.Value == ""
}

// Set sets the literal value for the multi-source value
func (v *MultiSourceValue) Set(value string) {
	v.Env = ""
	v.Path = ""
	v.Value = value
}
