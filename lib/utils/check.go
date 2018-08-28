package utils

import (
	"regexp"
	"strings"

	"github.com/gravitational/trace"
)

// Check is a simplistic email checker
func CheckEmail(email string) error {
	if email == "" {
		return trace.BadParameter("provide a valid email address")
	}
	if !strings.Contains(email, "@") {
		return trace.BadParameter("invalid email address '%v'", email)
	}
	return nil
}

// CheckUserName validates user name
func CheckUserName(name string) error {
	if name == "" {
		return trace.BadParameter("user name cannot be empty")
	}

	return nil
}

// CheckName makes sure that the provided string is a valid app name
func CheckName(name string) error {
	if strings.TrimSpace(name) == "" {
		return trace.BadParameter("app name can't be empty")
	}
	if !nameRe.MatchString(name) {
		return trace.BadParameter("app name may contain only alphanumeric characters, dashes and underscores")
	}
	return nil
}

// nameRe is a regular expression used to validate app name
var nameRe = regexp.MustCompile(`^[a-zA-Z0-9_-]+$`)
