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

package schema

import (
	"bytes"
	"encoding/base64"
	"fmt"
	"io/ioutil"
	"mime"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/gravitational/trace"
)

// ProcessMultiSourceValues replaces manifest fields that refer to files (via file:// schema)
// or internet resources (via http:// scheme) with their literal values (i.e. file/downloaded
// content).
//
// The following fields can be filepaths/URLs:
//   .releaseNotes
//   .logo
//   .installer.eula.source
//   .installer.flavors.description
//   .hooks.*.job
//   .webConfig
func ProcessMultiSourceValues(manifest *Manifest, manifestPath string) error {
	err := processText(&manifest.ReleaseNotes, manifestPath)
	if err != nil {
		return trace.Wrap(err)
	}

	err = processImage(&manifest.Logo, manifestPath)
	if err != nil {
		return trace.Wrap(err)
	}

	if manifest.Installer != nil {
		err = processText(&manifest.Installer.EULA.Source, manifestPath)
		if err != nil {
			return trace.Wrap(err)
		}

		err = processText(&manifest.Installer.Flavors.Description, manifestPath)
		if err != nil {
			return trace.Wrap(err)
		}
	}

	for i, profile := range manifest.NodeProfiles {
		for j := range profile.Requirements.CustomChecks {
			err = processText(&manifest.NodeProfiles[i].Requirements.CustomChecks[j].Script, manifestPath)
			if err != nil {
				return trace.Wrap(err)
			}
		}
	}

	err = processText(&manifest.WebConfig, manifestPath)
	if err != nil {
		return trace.Wrap(err)
	}

	if manifest.Hooks != nil {
		for _, hook := range manifest.Hooks.AllHooks() {
			err = processText(&hook.Job, manifestPath)
			if err != nil {
				return trace.Wrap(err)
			}
		}
	}

	return nil
}

// ExpandEnvVars does environment variables interpolation on manifest body.
//
// Environment variables have format ${VARNAME}.
func ExpandEnvVars(manifest []byte) []byte {
	replaceFn := func(match []byte) []byte {
		varName := string(bytes.Trim(match, "${}"))
		return []byte(os.Getenv(varName))
	}
	return reEnvVar.ReplaceAllFunc(manifest, replaceFn)
}

// processText replaces the value of "v" with the contents of the file
// or downloaded content, or does not change it if it's neither "file://"
// nor "http://"
func processText(v *string, manifestPath string) error {
	var data []byte
	var err error

	if strings.HasPrefix(*v, "file://") {
		data, _, err = valueFromFile(*v, manifestPath)
	} else if strings.HasPrefix(*v, "http://") || strings.HasPrefix(*v, "https://") {
		data, _, err = valueFromHTTP(*v)
	} else {
		return nil
	}

	if err != nil {
		return trace.Wrap(err)
	}

	*v = string(data)
	return nil
}

// processImage replaces the value of "v" with the contents of the image
// file or downloaded image in the web page friendly format, or does not
// change it if it's neither "file://" nor "http://"
func processImage(v *string, manifestPath string) error {
	var data []byte
	var mime string
	var err error

	if strings.HasPrefix(*v, "file://") {
		data, mime, err = valueFromFile(*v, manifestPath)
	} else if strings.HasPrefix(*v, "http://") || strings.HasPrefix(*v, "https://") {
		data, mime, err = valueFromHTTP(*v)
	} else {
		return nil
	}

	if err != nil {
		return trace.Wrap(err)
	}

	if mime == "" {
		mime = http.DetectContentType(data)
	}
	if !strings.HasPrefix(mime, "image/") {
		return trace.BadParameter(
			`invalid MIME type %q for %v, expected "image/*"`, mime, *v)
	}

	*v = encodeLogo(mime, data)
	return nil
}

// valueFromFile returns the contents of the file at the provided path
// and its MIME type
//
// If the path is absolute, it is used as-is, otherwise it is considered
// relative to the specified base path
func valueFromFile(path, basePath string) (data []byte, mimeType string, err error) {
	path = strings.TrimPrefix(path, "file://")
	if !filepath.IsAbs(path) {
		path = filepath.Join(filepath.Dir(basePath), path)
	}
	data, err = ioutil.ReadFile(path)
	if err != nil {
		return nil, "", trace.Wrap(err)
	}
	return data, mime.TypeByExtension(filepath.Ext(path)), nil
}

// valueFromHTTP returns the content downloaded from the provided URL
// and its MIME type
func valueFromHTTP(url string) (data []byte, mimeType string, err error) {
	//nolint:gosec,noctx // context will be added in a separate PR
	response, err := http.Get(url)
	if err != nil {
		return nil, "", trace.Wrap(err)
	}
	defer response.Body.Close()
	data, err = ioutil.ReadAll(response.Body)
	if err != nil {
		return nil, "", trace.Wrap(err)
	}
	return data, response.Header.Get("Content-Type"), nil
}

// encodeLogo encodes the provided image data with the specified mime type into a string
// in a format that can be embedded into web pages (e.g. <img> tag)
//
// The encoded string has the following format: "data:image/svg+xml;base64,PD94bWwgdmV..."
func encodeLogo(mime string, data []byte) string {
	return fmt.Sprintf("data:%v;base64,%v", mime, base64.StdEncoding.EncodeToString(data))
}

// reEnvVar matches an environment variable reference, e.g. $(ENVVAR)
var reEnvVar = regexp.MustCompile(`\$\{[A-Za-z0-9_]+\}`)
