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

package systeminfo

import (
	"strings"
	"testing"

	"github.com/gravitational/gravity/lib/storage"
	"github.com/stretchr/testify/assert"
)

func TestResolvParser(t *testing.T) {
	var tests = []struct {
		in  string
		out *storage.ResolvConf
	}{
		{
			`
			domain domain.com
			nameserver 1.1.1.1
			`,
			&storage.ResolvConf{
				Ndots:    1,
				Timeout:  5,
				Attempts: 2,
				Domain:   "domain.com",
				Search:   []string{"domain.com"},
				Servers:  []string{"1.1.1.1"},
			},
		},
		{
			`
			# domain domain.com
			nameserver 1.1.1.1
			`,
			&storage.ResolvConf{
				Ndots:    1,
				Timeout:  5,
				Attempts: 2,
				Servers:  []string{"1.1.1.1"},
			},
		},
		{
			`
			# domain domain.com
			nameserver 1.1.1.1`, // <- missing newline at EOF
			&storage.ResolvConf{
				Ndots:    1,
				Timeout:  5,
				Attempts: 2,
				Servers:  []string{"1.1.1.1"},
			},
		},
		{
			`
			nameserver 1.1.1.1 # inline comment
			nameserver 2.2.2.2 ; inline comment
			domain domain.com
			search a.com b.com c.com 
			options ndots:2 timeout:1 attempts:5 rotate
			`,
			&storage.ResolvConf{
				Ndots:    2,
				Timeout:  1,
				Attempts: 5,
				Servers:  []string{"1.1.1.1", "2.2.2.2"},
				Domain:   "domain.com",
				Search:   []string{"a.com", "b.com", "c.com"},
				Rotate:   true,
			},
		},
		{
			`
			options ndots:-1
			options timeout:-5
			options attempts:-5
			options unknownoption
			`,
			&storage.ResolvConf{
				Ndots:      1,
				Timeout:    1,
				Attempts:   1,
				UnknownOpt: true,
			},
		},
	}

	for _, tt := range tests {
		r, err := ResolvFromReader(strings.NewReader(tt.in))
		assert.Nil(t, err)
		assert.Equal(t, r, tt.out)
	}

}
