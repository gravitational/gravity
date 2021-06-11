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

// IdentifierExpr is identifier expression
type IdentifierExpr string

// String serializes identifier expression into format parsed by rules engine
func (i IdentifierExpr) String() string {
	return string(i)
}

var (
	// ResourceNameExpr is identifier that specifies resource name
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

// String returns function call expression used in rules
func (i ContainsExpr) String() string {
	return fmt.Sprintf("contains(%v, %v)", i.Left, i.Right)
}
