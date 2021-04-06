package schema

import (
	"strings"

	"github.com/gravitational/trace"
)

// NewContext creates a new Context struct from the specified label
func NewContext(label string) (c Context, err error) {
	if len(label) == 0 {
		return c, nil
	}
	con := strings.SplitN(label, ":", 4)
	if len(con) < 3 {
		return c, trace.BadParameter("invalid label")
	}
	c.User = con[0]
	c.Role = con[1]
	c.Type = con[2]
	if len(con) > 3 {
		c.Level = con[3]
	}
	return c, nil
}

// Context represents a SELinux label
type Context struct {
	// User specifies the SELinux user
	User string
	// Role specifies the SELinux role
	Role string
	// Type specifies the SELinux resource type
	Type string
	// Level specifies the SELinux MCS/MLS security level
	Level string
}
