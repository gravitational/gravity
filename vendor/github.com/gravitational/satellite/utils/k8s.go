/*
Copyright 2020 Gravitational, Inc.

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
	"k8s.io/apimachinery/pkg/labels"
)

// MustLabelSelector panics if the provided error is not nil. Returns
// the labels.Selector.
func MustLabelSelector(selector labels.Selector, err error) labels.Selector {
	if err != nil {
		panic(err)
	}
	return selector
}
