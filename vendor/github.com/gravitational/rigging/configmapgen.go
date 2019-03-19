/*
Copyright 2016 The Kubernetes Authors All rights reserved.

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

package rigging

import (
	"io/ioutil"
	"os"
	"path"
	"strings"

	"github.com/gravitational/trace"
	"k8s.io/api/core/v1"
)

// GenerateConfigMap returns a configMap using the specified parameters.
func GenerateConfigMap(name string, namespace string, fromFile []string, fromLiteral []string) (*v1.ConfigMap, error) {
	if name == "" {
		return nil, trace.BadParameter("missing parameter Name")
	}
	configMap := &v1.ConfigMap{}
	configMap.Name = name
	configMap.Kind = KindConfigMap
	configMap.APIVersion = v1.SchemeGroupVersion.String()
	configMap.Namespace = namespace
	configMap.Data = map[string]string{}
	if err := handleConfigMapFromFileSources(configMap, fromFile); err != nil {
		return nil, trace.Wrap(err)
	}
	if err := handleConfigMapFromLiteralSources(configMap, fromLiteral); err != nil {
		return nil, trace.Wrap(err)
	}
	return configMap, nil
}

// handleConfigMapFromLiteralSources adds the specified literal source
// information into the provided configMap.
func handleConfigMapFromLiteralSources(configMap *v1.ConfigMap, literalSources []string) error {
	for _, literalSource := range literalSources {
		key, value, err := parseLiteralSource(literalSource)
		if err != nil {
			return err
		}
		err = addKeyFromLiteral(configMap, key, value)
		if err != nil {
			return err
		}
	}
	return nil
}

// handleConfigMapFromFileSources adds the specified file source information
// into the provided configMap
func handleConfigMapFromFileSources(configMap *v1.ConfigMap, fileSources []string) error {
	for _, fileSource := range fileSources {
		key, filePath, err := parseFileSource(fileSource)
		if err != nil {
			return err
		}
		info, err := os.Stat(filePath)
		if err != nil {
			return trace.ConvertSystemError(err)
		}
		if info.IsDir() {
			if strings.Contains(fileSource, "=") {
				return trace.BadParameter("cannot give a key name for a directory path.")
			}
			fileList, err := ioutil.ReadDir(filePath)
			if err != nil {
				return trace.ConvertSystemError(err)
			}
			for _, item := range fileList {
				itemPath := path.Join(filePath, item.Name())
				if item.Mode().IsRegular() {
					key = item.Name()
					err = addKeyFromFile(configMap, key, itemPath)
					if err != nil {
						return trace.Wrap(err)
					}
				}
			}
		} else {
			err = addKeyFromFile(configMap, key, filePath)
			if err != nil {
				return trace.Wrap(err)
			}
		}
	}

	return nil
}

// addKeyFromFile adds a key with the given name to a ConfigMap, populating
// the value with the content of the given file path, or returns an error.
func addKeyFromFile(configMap *v1.ConfigMap, key, filePath string) error {
	data, err := ioutil.ReadFile(filePath)
	if err != nil {
		return err
	}
	return addKeyFromLiteral(configMap, key, string(data))
}

// addKeyFromLiteral adds the given key and data to the given config map,
// returning an error if the key is not valid or if the key already exists.
func addKeyFromLiteral(configMap *v1.ConfigMap, keyName, data string) error {
	if _, entryExists := configMap.Data[keyName]; entryExists {
		return trace.BadParameter("cannot add key %s, another key by that name already exists: %v.", keyName, configMap.Data)
	}
	configMap.Data[keyName] = data
	return nil
}

// parseLiteralSource parses the source as key=val pair
func parseLiteralSource(source string) (keyName, value string, err error) {
	// leading equal is invalid
	if strings.Index(source, "=") == 0 {
		return "", "", trace.BadParameter("invalid literal source %v, expected key=value", source)
	}
	// split after the first equal (so values can have the = character)
	items := strings.SplitN(source, "=", 2)
	if len(items) != 2 {
		return "", "", trace.BadParameter("invalid literal source %v, expected key=value", source)
	}

	return items[0], items[1], nil
}

// parseFileSource parses the source given. Acceptable formats include:
//
// 1.  source-path: the basename will become the key name
// 2.  source-name=source-path: the source-name will become the key name and source-path is the path to the key file
//
// Key names cannot include '='.
func parseFileSource(source string) (keyName, filePath string, err error) {
	numSeparators := strings.Count(source, "=")
	switch {
	case numSeparators == 0:
		return path.Base(source), source, nil
	case numSeparators == 1 && strings.HasPrefix(source, "="):
		return "", "", trace.BadParameter("key name for file path %v missing.", strings.TrimPrefix(source, "="))
	case numSeparators == 1 && strings.HasSuffix(source, "="):
		return "", "", trace.BadParameter("file path for key name %v missing.", strings.TrimSuffix(source, "="))
	case numSeparators > 1:
		return "", "", trace.BadParameter("key names or file paths cannot contain '='.")
	default:
		components := strings.Split(source, "=")
		return components[0], components[1], nil
	}
}
