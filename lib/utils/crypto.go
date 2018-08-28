package utils

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"

	"github.com/gravitational/gravity/lib/defaults"

	"github.com/gravitational/trace"
)

// GeneratePrivateKeyPEM generates and returns PEM serialzed
func GeneratePrivateKeyPEM() ([]byte, error) {
	key, err := rsa.GenerateKey(rand.Reader, defaults.RSAPrivateKeyBits)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	keyPEM := pem.EncodeToMemory(
		&pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(key)})
	if keyPEM == nil {
		return nil, trace.BadParameter("failed to encode key")
	}
	return keyPEM, nil
}
