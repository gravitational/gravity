// Copyright 2021 Gravitational Inc.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package environment

import (
	"github.com/gravitational/gravity/e/lib/ops/client"
	"github.com/gravitational/gravity/lib/localenv"

	"github.com/gravitational/trace"
)

// Remote extends the RemoteEnvironment from open-source
type Remote struct {
	// RemoteEnvironment is the wrapped open-source remote env
	*localenv.RemoteEnvironment
	// Operator is the enterprise ops web client
	Operator *client.Client
}

// LoginRemote creates new remote environment
func LoginRemote(url, token string) (*Remote, error) {
	ossRemote, err := localenv.LoginRemote(url, token)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return &Remote{
		RemoteEnvironment: ossRemote,
		Operator:          client.New(ossRemote.Operator),
	}, nil
}
