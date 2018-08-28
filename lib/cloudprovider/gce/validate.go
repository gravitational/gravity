package gce

import (
	"regexp"

	"github.com/gravitational/trace"
)

// ValidateTag validates the tag value for conformance
func ValidateTag(tag string) error {
	if len(tag) == 0 {
		return trace.BadParameter("tag value cannot be empty")
	}
	var errors []error
	if len(tag) > maxTagLength {
		errors = append(errors,
			trace.BadParameter("tag value cannot be longer than %v characters", maxTagLength))
	}
	if !tagRegex.MatchString(tag) && !tagSingleCharacterRegex.MatchString(tag) {
		errors = append(errors,
			trace.BadParameter("tag value can only contain lowercase letters, numeric characters, and dashes.\n"+
				"Tag value must start and end with either a number or a lowercase character"))
	}
	return trace.NewAggregate(errors...)
}

// maxTagLength limits the length of the tag value.
// Tag value restrictions: see https://cloud.google.com/vpc/docs/add-remove-network-tags
const maxTagLength = 63

// tagRegex defines the syntax for tag values.
// Tag values can only contain lowercase letters, numeric characters, and dashes.
// Tag values must start and end with either a number or a lowercase character.
var tagRegex = regexp.MustCompile(`^[a-z0-9]+[a-z0-9\-]*[a-z0-9]+$`)

var tagSingleCharacterRegex = regexp.MustCompile(`^[a-z0-9]$`)
