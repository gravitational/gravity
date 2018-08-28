package storage

import (
	"fmt"
	"strings"

	"github.com/gravitational/gravity/lib/constants"
)

// AssignKubernetesGroupsExpr constructs function expression used in rules specifications
// that assigns kubernetes groups to the current user
type AssignKubernetesGroupsExpr struct {
	// Groups is a list of groups to assign
	Groups StringsExpr
}

// String returns function call expression used in rules
func (a AssignKubernetesGroupsExpr) String() string {
	return fmt.Sprintf(`%v(%v)`, constants.AssignKubernetesGroupsFnName, a.Groups)
}

// Expr is an expression
type Expr interface {
	// String serializes expression into format parsed by rules engine
	// (golang based syntax)
	String() string
}

// IdentiferExpr is identifer expression
type IdentifierExpr string

// String serializes identifer expression into format parsed by rules engine
func (i IdentifierExpr) String() string {
	return string(i)
}

var (
	// ResourceNameExpr is identifer that specifies resource name
	ResourceNameExpr = IdentifierExpr("resource.metadata.name")
)

// StringExpr is a string expression
type StringExpr string

func (s StringExpr) String() string {
	return fmt.Sprintf("%q", string(s))
}

// StringsExpr is a slice of strings
type StringsExpr []string

func (s StringsExpr) String() string {
	var out []string
	for _, val := range s {
		out = append(out, fmt.Sprintf("%q", val))
	}
	return strings.Join(out, ",")
}

// EqualsExpr constructs function expression used in rules specifications
// that checks if one value is equal to another
// e.g. equals("a", "b") where Left is "a" and right is "b"
type EqualsExpr struct {
	// Left is a left argument of Equals expression
	Left Expr
	// Value to check
	Right Expr
}

// String returns function call expression used in rules
func (i EqualsExpr) String() string {
	return fmt.Sprintf("equals(%v, %v)", i.Left, i.Right)
}

// ContainsExpr constructs function expression used in rules specifications
// that checks if one value contains the other, e.g.
// contains([]string{"a"}, "b") where left is []string{"a"} and right is "b"
type ContainsExpr struct {
	// Left is a left argument of Contains expression
	Left Expr
	// Right is a right argument of Contains expression
	Right Expr
}

// String rturns function call expression used in rules
func (i ContainsExpr) String() string {
	return fmt.Sprintf("contains(%v, %v)", i.Left, i.Right)
}
