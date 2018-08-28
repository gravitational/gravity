package utils

import (
	"io"

	log "github.com/sirupsen/logrus"
	"github.com/gravitational/trace"
	"golang.org/x/crypto/openpgp"
)

// EncryptPGP returns a stream with "data" encrypted by the provided passphrase
func EncryptPGP(data io.Reader, passphrase string) (io.ReadCloser, error) {
	pr, pw := io.Pipe()

	go func() {
		w, err := openpgp.SymmetricallyEncrypt(pw, []byte(passphrase), &openpgp.FileHints{
			IsBinary: true,
		}, nil)
		if err != nil {
			log.Infof(trace.DebugReport(err))
			pw.CloseWithError(err)
			return
		}

		_, err = io.Copy(w, data)
		w.Close()
		pw.CloseWithError(err)
	}()

	return pr, nil
}

// DecryptPGP returns a stream with "data" decrypted by the provided passphrase
func DecryptPGP(data io.Reader, passphrase string) (io.Reader, error) {
	promptFn := func(_ []openpgp.Key, _ bool) ([]byte, error) {
		return []byte(passphrase), nil
	}

	md, err := openpgp.ReadMessage(data, nil, promptFn, nil)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return md.UnverifiedBody, nil
}
