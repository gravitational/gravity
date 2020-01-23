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

package utils

import (
	"io"

	"github.com/gravitational/trace"
	log "github.com/sirupsen/logrus"
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
			pw.CloseWithError(err) //nolint:errcheck
			return
		}

		_, err = io.Copy(w, data)
		w.Close()
		pw.CloseWithError(err) //nolint:errcheck
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
