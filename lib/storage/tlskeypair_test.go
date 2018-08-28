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
	teleutils "github.com/gravitational/teleport/lib/utils"
	"gopkg.in/check.v1"
)

type TLSKeyPairSuite struct{}

var _ = check.Suite(&TLSKeyPairSuite{})

func (s *TLSKeyPairSuite) TestTLSKeyPair(c *check.C) {

	keyPair, err := teleutils.GenerateSelfSignedCert([]string{"test.localhost"})
	c.Assert(err, check.IsNil)

	pair := NewTLSKeyPair(keyPair.Cert, keyPair.PrivateKey)
	c.Assert(pair.CheckAndSetDefaults(), check.IsNil)

	// key valid, cert invalid
	pair = NewTLSKeyPair([]byte("bad cert"), keyPair.PrivateKey)
	c.Assert(pair.CheckAndSetDefaults(), check.NotNil)

	// cert valid, key invalid
	pair = NewTLSKeyPair(keyPair.Cert, []byte("bad key"))
	c.Assert(pair.CheckAndSetDefaults(), check.NotNil)
}
