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

package resources

import (
	"encoding/json"
	"io"

	"github.com/gravitational/gravity/lib/utils"

	"github.com/gravitational/trace"
	log "github.com/sirupsen/logrus"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	serializer "k8s.io/apimachinery/pkg/runtime/serializer/json"
	"k8s.io/apimachinery/pkg/util/yaml"
	"k8s.io/client-go/kubernetes/scheme"
)

// Decode decodes kubernetes resources from the specified io.Reader
func Decode(r io.Reader, options ...DecodeOption) (resource *Resource, err error) {
	decoder, _, encoding := newCodec(r, options...)
	var objects []runtime.Object
L:
	for {
		object, err := decoder.Decode()
		if err != nil {
			if trace.Unwrap(err) == io.EOF {
				break L
			}
			if utils.IsContinueError(err) {
				log.Warn(err)
				continue L
			}
			return nil, trace.Wrap(err, "decode failure: %v", err)
		}
		objects = append(objects, object)
	}
	return &Resource{Objects: objects, encoding: encoding}, nil
}

// DecodeOption is a functional argument for decoding
type DecodeOption func(*universalDecoder)

type encoding int

const (
	// JSON encoding
	jsonEncoding = iota
	// YAML encoding
	yamlEncoding
)

// Resource combines a set of kubernetes resources and a means to serialize them
// in the original format (JSON or YAML)
type Resource struct {
	Objects  []runtime.Object
	encoding encoding
}

// NewResource creates a new resource object that can encode objects it
// was initialized with in YAML format
func NewResource(objects ...runtime.Object) Resource {
	return Resource{
		Objects:  objects,
		encoding: yamlEncoding,
	}
}

// Encode encodes contained resources in the original format (JSON or YAML)
func (r Resource) Encode(w io.Writer) error {
	return Encode(r.Objects, r.encoding, w)
}

// newCodec creates decoder/encoder pair for the specified reader r
func newCodec(r io.Reader, options ...DecodeOption) (decoder *universalDecoder, encoder *universalEncoder, encoding encoding) {
	buffer, _, isJSON := yaml.GuessJSONStream(r, bufferSize)
	decoder = newUniversalDecoder(buffer)
	for _, o := range options {
		o(decoder)
	}
	if isJSON {
		encoding = jsonEncoding
		pretty := true
		encoder = &universalEncoder{
			newFramer: serializer.Framer.NewFrameWriter,
			Encoder:   serializer.NewSerializer(nil, nil, nil, pretty),
		}
	} else {
		encoding = yamlEncoding
		encoder = &universalEncoder{
			newFramer: serializer.YAMLFramer.NewFrameWriter,
			Encoder:   serializer.NewYAMLSerializer(nil, nil, nil),
		}
	}
	return decoder, encoder, encoding
}

// newUniversalDecoder creates an instance of decoder that can
// decode both YAML and JSON streams
func newUniversalDecoder(r io.Reader) *universalDecoder {
	streamDecoder := yaml.NewYAMLOrJSONDecoder(r, bufferSize)
	decoder := scheme.Codecs.UniversalDeserializer()
	return &universalDecoder{
		streamDecoder: streamDecoder,
		Decoder:       decoder,
	}
}

// universalDecoder is a decoder for resources in either JSON or YAML format
type universalDecoder struct {
	runtime.Decoder
	streamDecoder *yaml.YAMLOrJSONDecoder
}

// Decode obtains the next object from the stream.
// Returns io.EOF upon exhausting the stream.
func (r *universalDecoder) Decode() (runtime.Object, error) {
	var unk Unknown
	if err := r.streamDecoder.Decode(&unk); err != nil {
		return nil, trace.Wrap(err)
	}
	if len(unk.Raw) == 0 {
		return nil, utils.Continue("skip empty object")
	}
	if unk.Kind == "" {
		// Return unparsed for resources that are pass-through
		return &unk, nil
	}
	object, err := runtime.Decode(r.Decoder, unk.Raw)
	if err != nil {
		if runtime.IsNotRegisteredError(trace.Unwrap(err)) {
			return &unk, nil
		}
		return nil, trace.Wrap(err)
	}
	return object, nil
}

// universalEncoder is an encoder that can encode in either YAML or JSON format
type universalEncoder struct {
	runtime.Encoder
	newFramer
}

// newFramer is a constructor that returns a new stream framer
// for a specific stream type.
type newFramer func(io.Writer) io.Writer

// Encode encodes the specified objects into the given writer
func Encode(objects []runtime.Object, encoding encoding, w io.Writer) error {
	var encoder universalEncoder
	switch encoding {
	case yamlEncoding:
		encoder = universalEncoder{
			newFramer: serializer.YAMLFramer.NewFrameWriter,
			Encoder:   serializer.NewYAMLSerializer(nil, nil, nil),
		}
	case jsonEncoding:
		encoder = universalEncoder{
			newFramer: serializer.Framer.NewFrameWriter,
			Encoder:   serializer.NewSerializer(nil, nil, nil, true),
		}
	default:
		return trace.BadParameter("unknown encoding: %v", encoding)
	}

	if len(objects) > 1 {
		// Use framer to combine multiple document into a single file
		w = encoder.newFramer(w)
	}

	for _, object := range objects {
		err := encoder.Encode(object, w)
		if err != nil {
			return trace.Wrap(err)
		}
	}
	return nil
}

// ToUnknown attempts to convert the provided generic object to an unknown resource.
func ToUnknown(object runtime.Object) (*Unknown, error) {
	bytes, err := json.Marshal(object)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	var unk Unknown
	if err := json.Unmarshal(bytes, &unk); err != nil {
		return nil, trace.Wrap(err)
	}
	return &unk, nil
}

// Unknown represents an unparsed Kubernetes resource with an interpreted
// TypeMeta and ObjectMeta fields which are used for type recognition.
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
type Unknown struct {
	unknown
	Raw json.RawMessage `json:",inline"`
}

type unknown struct {
	runtime.TypeMeta
	Metadata metav1.ObjectMeta `json:"metadata"`
}

// GetObjectKind returns the ObjectKind for this Unknown.
// Implements runtime.Object
func (r *Unknown) GetObjectKind() schema.ObjectKind {
	return &r.TypeMeta
}

// UnmarshalJSON consumes the specified data as a binary blob w/o interpreting it
func (r *Unknown) UnmarshalJSON(data []byte) error {
	if err := json.Unmarshal(data, &r.unknown); err != nil {
		return trace.Wrap(err)
	}
	if err := r.Raw.UnmarshalJSON(data); err != nil {
		return trace.Wrap(err)
	}
	return nil
}

// MarshalJSON returns the raw message
func (r *Unknown) MarshalJSON() ([]byte, error) {
	return r.Raw.MarshalJSON()
}

const bufferSize = 4096
