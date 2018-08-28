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
