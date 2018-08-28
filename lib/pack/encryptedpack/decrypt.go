package encryptedpack

import (
	log "github.com/sirupsen/logrus"
	"github.com/gravitational/trace"

	"github.com/gravitational/gravity/lib/loc"
	"github.com/gravitational/gravity/lib/pack"
)

// DecryptPackage decrypts the specified package in the specified package service
func DecryptPackage(p pack.PackageService, loc loc.Locator, encryptionKey string) error {
	envelope, data, err := p.ReadPackage(loc)
	if err != nil {
		return trace.Wrap(err)
	}

	if !envelope.Encrypted {
		log.Infof("%v is not encrypted, nothing to do", loc)
		return nil
	}

	log.Infof("decrypting %v", loc)

	// wrap the package service into "encrypted pack" to decrypt the package
	encryptedPack := New(p, encryptionKey)
	envelope, data, err = encryptedPack.ReadPackage(loc)
	if err != nil {
		return trace.Wrap(err)
	}

	options := envelope.Options()
	options = append(options, pack.WithEncrypted(false))

	// update the decrypted package in the original package service
	_, err = p.UpsertPackage(loc, data, options...)
	return trace.Wrap(err)
}
