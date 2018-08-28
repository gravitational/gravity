// Copyright 2016 Gravitational Inc.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//    http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package rigging

import (
	"fmt"
	"strings"
	"time"

	"github.com/gravitational/trace"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

type ChangesetList struct {
	metav1.TypeMeta `json:",inline"`
	// Standard list metadata
	// More info: http://releases.k8s.io/HEAD/docs/devel/api-conventions.md#metadata
	metav1.ListMeta `json:"metadata,omitempty"`
	// Items is a list of third party objects
	Items []ChangesetResource `json:"items"`
}

func (tr *ChangesetList) GetObjectKind() schema.ObjectKind {
	return &tr.TypeMeta
}

type ChangesetResource struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`
	Spec              ChangesetSpec `json:"spec"`
}

func (tr *ChangesetResource) GetObjectKind() schema.ObjectKind {
	return &tr.TypeMeta
}

func (tr *ChangesetResource) String() string {
	return fmt.Sprintf("namespace=%v, name=%v, operations=%v)", tr.Namespace, tr.Name, len(tr.Spec.Items))
}

type ChangesetSpec struct {
	Status string          `json:"status"`
	Items  []ChangesetItem `json:"items"`
}

type ChangesetItem struct {
	From              string    `json:"from"`
	To                string    `json:"to"`
	UID               string    `json:"uid"`
	Status            string    `json:"status"`
	CreationTimestamp time.Time `json:"time"`
}

type OperationInfo struct {
	From *ResourceHeader
	To   *ResourceHeader
}

func (o *OperationInfo) Kind() string {
	if o.From != nil && o.From.Kind != "" {
		return o.From.Kind
	}
	if o.To != nil && o.To.Kind != "" {
		return o.To.Kind
	}
	return ""
}

func (o *OperationInfo) String() string {
	if o.From != nil && o.To == nil {
		return fmt.Sprintf("delete %v %v", o.From.Kind, formatMeta(o.From.ObjectMeta))
	}
	if o.From != nil && o.To != nil {
		return fmt.Sprintf("update %v %v", o.To.Kind, formatMeta(o.To.ObjectMeta))
	}
	if o.From == nil && o.To != nil {
		return fmt.Sprintf("upsert %v %v", o.To.Kind, formatMeta(o.To.ObjectMeta))
	}
	return "invalid operation: both resources cannot be empty"
}

// GetOperationInfo returns operation information
func GetOperationInfo(item ChangesetItem) (*OperationInfo, error) {
	var info OperationInfo
	if item.From != "" {
		from, err := ParseResourceHeader(strings.NewReader(item.From))
		if err != nil {
			return nil, trace.Wrap(err)
		}
		info.From = from
	}
	if item.To != "" {
		to, err := ParseResourceHeader(strings.NewReader(item.To))
		if err != nil {
			return nil, trace.Wrap(err)
		}
		info.To = to
	}
	return &info, nil
}
