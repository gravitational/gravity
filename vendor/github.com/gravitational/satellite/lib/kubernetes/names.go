package kubernetes

import (
	"fmt"

	utilrand "k8s.io/apimachinery/pkg/util/rand"
)

// NameGenerator generates names for objects. Some backends may have more information
// available to guide selection of new names and this interface hides those details.
type NameGenerator interface {
	// GenerateName generates a valid name from the base name, adding a random suffix to the
	// the base. If base is valid, the returned name must also be valid. The generator is
	// responsible for knowing the maximum valid name length.
	GenerateName(base string) string
}

// simpleNameGenerator generates random names.
type simpleNameGenerator struct{}

// SimpleNameGenerator is a generator that returns the name plus a random suffix of five alphanumerics
// when a name is requested. The string is guaranteed to not exceed the length of a standard Kubernetes
// name (63 characters)
var SimpleNameGenerator NameGenerator = simpleNameGenerator{}

const (
	// TODO: make this flexible for non-core resources with alternate naming rules.
	maxNameLength          = 63
	randomLength           = 5
	maxGeneratedNameLength = maxNameLength - randomLength
)

func (simpleNameGenerator) GenerateName(base string) string {
	if len(base) > maxGeneratedNameLength {
		base = base[:maxGeneratedNameLength]
	}
	return fmt.Sprintf("%s%s", base, utilrand.String(randomLength))
}
