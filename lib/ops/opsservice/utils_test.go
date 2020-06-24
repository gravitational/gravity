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

package opsservice

import (
	"github.com/gravitational/gravity/lib/utils"

	"gopkg.in/check.v1"
)

type UtilsSuite struct {
}

var _ = check.Suite(&UtilsSuite{})

func (s *UtilsSuite) TestChownExpr(c *check.C) {
	tests := []struct {
		uid     *int
		gid     *int
		result  string
		comment string
	}{
		{
			uid:     utils.IntPtr(1000),
			result:  "1000",
			comment: "only uid is specified",
		},
		{
			gid:     utils.IntPtr(2000),
			result:  ":2000",
			comment: "only gid is specified",
		},
		{
			uid:     utils.IntPtr(1000),
			gid:     utils.IntPtr(2000),
			result:  "1000:2000",
			comment: "both uid and gid are specified",
		},
	}
	for _, t := range tests {
		c.Assert(chownExpr(t.uid, t.gid), check.Equals, t.result,
			check.Commentf(t.comment))
	}
}
