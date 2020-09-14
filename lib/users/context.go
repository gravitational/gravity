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

package users

import (
	"fmt"

	"github.com/gravitational/gravity/lib/constants"
	"github.com/gravitational/trace"

	teleservices "github.com/gravitational/teleport/lib/services"
	log "github.com/sirupsen/logrus"
	"github.com/vulcand/predicate"
)

func init() {
	teleservices.SetActionsParserFn(NewActionsParser)
}

// Context is a context used in access rules
type Context struct {
	teleservices.Context
	// KubernetesGroups is  processed by action assignKubernetesGroups
	KubernetesGroups []string
}

// String returns user friendly representation of this context
func (ctx *Context) String() string {
	return fmt.Sprintf("user %v, resource: %v", ctx.User, ctx.Resource)
}

const (
	// UserIdentifier represents user registered identifier in the rules
	UserIdentifier = "user"
	// ResourceIdentifier represents resource registered identifer in the rules
	ResourceIdentifier = "resource"
)

// NewActionsParser returns standard parser for 'actions' section in access rules
func NewActionsParser(ctx teleservices.RuleContext) (predicate.Parser, error) {
	return predicate.NewParser(predicate.Def{
		Operators: predicate.Operators{},
		Functions: map[string]interface{}{
			"log":                                  teleservices.NewLogActionFn(ctx),
			constants.AssignKubernetesGroupsFnName: NewAssignKubernetesGroupsActionFn(ctx),
		},
		GetIdentifier: ctx.GetIdentifier,
		GetProperty:   predicate.GetStringMapValue,
	})
}

// ExtractKubeGroups returns a list of Kubernetes groups extracted from
// the provided assignKubernetesGroups action string
func ExtractKubeGroups(action string) ([]string, error) {
	ctx := &Context{}
	parser, err := NewActionsParser(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	assign, err := parser.Parse(action)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	assignFn, ok := assign.(predicate.BoolPredicate)
	if ok {
		assignFn()
		return ctx.KubernetesGroups, nil
	}
	return []string{}, nil
}

// NewAssignKubernetesGroupsActionFn creates assgin functions
func NewAssignKubernetesGroupsActionFn(ctx teleservices.RuleContext) interface{} {
	return (&AssignKubernetesGroupsAction{ctx: ctx}).Assign
}

// AssignKubernetesGroupsAction represents action that will
// assign kubernetes groups when called
type AssignKubernetesGroupsAction struct {
	ctx teleservices.RuleContext
}

// Assign assigns kubernetes groups to the context groups
func (l *AssignKubernetesGroupsAction) Assign(groups ...interface{}) predicate.BoolPredicate {
	return func() bool {
		ctx, ok := l.ctx.(*Context)
		if !ok {
			return false
		}
		ctx.KubernetesGroups = []string{}
		for _, igroup := range groups {
			switch group := igroup.(type) {
			case string:
				ctx.KubernetesGroups = append(ctx.KubernetesGroups, group)
			case []string:
				ctx.KubernetesGroups = append(ctx.KubernetesGroups, group...)
			default:
				log.Errorf("assignKubernetesGroup(), unsupported argument type: %T", group)
			}
		}
		return true
	}
}
