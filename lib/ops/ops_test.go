/*
Copyright 2020 Gravitational, Inc.

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

package ops

import (
	"github.com/gravitational/gravity/lib/storage"
	check "gopkg.in/check.v1"
)

type OperationsFilterSuite struct{}

var _ = check.Suite(&OperationsFilterSuite{})

func (s *OperationsFilterSuite) TestOperationsFilter(c *check.C) {
	tests := []struct {
		description string
		in          SiteOperations
		out         SiteOperations
		filter      OperationsFilter
	}{
		{
			description: "empty filter",
			in: []storage.SiteOperation{
				{
					ID: "op1",
				},
				{
					ID: "op2",
				},
			},
			out: []storage.SiteOperation{
				{
					ID: "op1",
				},
				{
					ID: "op2",
				},
			},
			filter: OperationsFilter{},
		},
		{
			description: "empty input",
			filter:      OperationsFilter{},
		},
		{
			description: "first (most recent is last)",
			in: []storage.SiteOperation{
				{
					ID: "op1",
				},
				{
					ID: "op2",
				},
			},
			out: []storage.SiteOperation{
				{
					ID: "op2",
				},
			},
			filter: OperationsFilter{
				First: true,
			},
		},
		{
			description: "last (most recent is last)",
			in: []storage.SiteOperation{
				{
					ID: "op1",
				},
				{
					ID: "op2",
				},
			},
			out: []storage.SiteOperation{
				{
					ID: "op1",
				},
			},
			filter: OperationsFilter{
				Last: true,
			},
		},
		{
			description: "finished",
			in: []storage.SiteOperation{
				{
					ID: "op1",
				},
				{
					ID:    "op2",
					State: OperationStateCompleted,
				},
				{
					ID:    "op3",
					State: OperationStateFailed,
				},
			},
			out: []storage.SiteOperation{
				{
					ID:    "op2",
					State: OperationStateCompleted,
				},
				{
					ID:    "op3",
					State: OperationStateFailed,
				},
			},
			filter: OperationsFilter{
				Finished: true,
			},
		},
		{
			description: "completed",
			in: []storage.SiteOperation{
				{
					ID: "op1",
				},
				{
					ID:    "op2",
					State: OperationStateCompleted,
				},
				{
					ID: "op3",
				},
			},
			out: []storage.SiteOperation{
				{
					ID:    "op2",
					State: OperationStateCompleted,
				},
			},
			filter: OperationsFilter{
				Complete: true,
			},
		},
		{
			description: "active",
			in: []storage.SiteOperation{
				{
					ID: "op1",
				},
				{
					ID:    "op2",
					State: OperationStateCompleted,
				},
				{
					ID: "op3",
				},
			},
			out: []storage.SiteOperation{
				{
					ID: "op1",
				},
				{
					ID: "op3",
				},
			},
			filter: OperationsFilter{
				Active: true,
			},
		},
		{
			description: "combined complete / last",
			in: []storage.SiteOperation{
				{
					ID: "op1",
				},
				{
					ID:    "op2",
					State: OperationStateCompleted,
				},
				{
					ID:    "op3",
					State: OperationStateCompleted,
				},
			},
			out: []storage.SiteOperation{
				{
					ID:    "op2",
					State: OperationStateCompleted,
				},
			},
			filter: OperationsFilter{
				Finished: true,
				Last:     true,
			},
		},
		{
			description: "type",
			in: []storage.SiteOperation{
				{
					ID: "op1",
				},
				{
					ID:    "op2",
					State: OperationStateCompleted,
					Type:  OperationUpdate,
				},
				{
					ID:    "op3",
					State: OperationStateFailed,
				},
			},
			out: []storage.SiteOperation{
				{
					ID:    "op2",
					State: OperationStateCompleted,
					Type:  OperationUpdate,
				},
			},
			filter: OperationsFilter{
				Types: []string{OperationUpdate},
			},
		},
		{
			description: "multiple types",
			in: []storage.SiteOperation{
				{
					ID: "op1",
				},
				{
					ID:    "op2",
					State: OperationStateCompleted,
					Type:  OperationUpdate,
				},
				{
					ID:    "op3",
					State: OperationStateFailed,
					Type:  OperationShrink,
				},
			},
			out: []storage.SiteOperation{
				{
					ID:    "op2",
					State: OperationStateCompleted,
					Type:  OperationUpdate,
				},
				{
					ID:    "op3",
					State: OperationStateFailed,
					Type:  OperationShrink,
				},
			},
			filter: OperationsFilter{
				Types: []string{OperationUpdate, OperationShrink},
			},
		},
	}

	for _, tt := range tests {
		c.Assert(tt.filter.Filter(tt.in), check.DeepEquals, tt.out, check.Commentf(tt.description))
	}
}
