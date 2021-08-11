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
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/dustin/go-humanize"
	teleservices "github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/trace"
)

// Capacity allows to define capacity in a human readable form (e.g. "10GB")
type Capacity uint64

// Bytes returns the number of bytes
func (c Capacity) Bytes() uint64 {
	return uint64(c)
}

// Megabytes returns the number of megabytes
func (c Capacity) Megabytes() uint64 {
	return c.Bytes() / 1000 / 1000
}

// String returns a human friendly capacity representation
func (c Capacity) String() string {
	return humanize.Bytes(uint64(c))
}

// MarshalJSON marshals capacity to a string in a human friendly form
func (c Capacity) MarshalJSON() ([]byte, error) {
	// marshal as a humanized string
	bytes, err := json.Marshal(humanize.Bytes(uint64(c)))
	return bytes, trace.Wrap(err)
}

// UnmarshalJSON unmarshals capacity from a human friendly form
func (c *Capacity) UnmarshalJSON(data []byte) error {
	// unmarshal as a string first
	var capacity string
	if err := json.Unmarshal(data, &capacity); err != nil {
		return trace.Wrap(err, "could not unmarshal %q to a string", string(data))
	}
	bytes, err := humanize.ParseBytes(capacity)
	if err != nil {
		return trace.Wrap(err, "could not parse %q as bytes", capacity)
	}
	*c = Capacity(bytes)
	return nil
}

// MustParseCapacity parses the provided string as capacity or panics
func MustParseCapacity(data string) Capacity {
	bytes, err := humanize.ParseBytes(data)
	if err != nil {
		panic(trace.Wrap(err))
	}
	return Capacity(bytes)
}

// TransferRate allows to define transfer rate in a human friendly form (e.g. "10MB/s")
type TransferRate uint64

// BytesPerSecond returns a number of bytes per second the transfer rate represents
func (r TransferRate) BytesPerSecond() uint64 {
	return uint64(r)
}

// String returns a human friendly formatted transfer rate
func (r TransferRate) String() string {
	return fmt.Sprintf("%v/s", humanize.Bytes(uint64(r)))
}

// MarshalJSON marshals transfer rate into a human friendly form
func (r TransferRate) MarshalJSON() ([]byte, error) {
	// marshal as a humanized string
	bytes, err := json.Marshal(fmt.Sprintf("%v/s", humanize.Bytes(uint64(r))))
	return bytes, trace.Wrap(err)
}

// UnmarshalJSON unmarshals transfer rate from a human friendly form
func (r *TransferRate) UnmarshalJSON(data []byte) error {
	// unmarshal as a string first
	var rate string
	if err := json.Unmarshal(data, &rate); err != nil {
		return trace.Wrap(err, "could not unmarshal %q to a string", string(data))
	}
	if !strings.HasSuffix(rate, "/s") {
		return trace.BadParameter("expected transfer rate as value/s but got %q", rate)
	}
	bytes, err := humanize.ParseBytes(strings.TrimSuffix(rate, "/s"))
	if err != nil {
		return trace.Wrap(err, "could not parse %q as transfer rate", rate)
	}
	*r = TransferRate(bytes)
	return nil
}

// MustParseTransferRate parses the provided data as a transfer rate or panics
func MustParseTransferRate(data string) TransferRate {
	if !strings.HasSuffix(data, "/s") {
		panic(trace.BadParameter("unsupported transfer rate format"))
	}
	bytes, err := humanize.ParseBytes(strings.TrimSuffix(data, "/s"))
	if err != nil {
		panic(trace.Wrap(err))
	}
	return TransferRate(bytes)
}

// Int64Ptr returns a pointer to an int64 with value v
func Int64Ptr(v int64) *int64 {
	return &v
}

// IntPtr returns a pointer to an int with value v
func IntPtr(v int) *int {
	return &v
}

// BoolValue returns the boolean value in v or false if it's nil
func BoolValue(v *bool) bool {
	return v != nil && *v
}

// BoolPtr returns a pointer to a bool with value v
func BoolPtr(v bool) *bool {
	return &v
}

// FormatBoolPtr formats the bool pointer value for output
func FormatBoolPtr(v *bool) string {
	if v == nil {
		return "<unset>"
	}
	return fmt.Sprint(*v)
}

// DurationPtr returns a pointer to the provided duration value
func DurationPtr(v time.Duration) *teleservices.Duration {
	d := teleservices.NewDuration(v)
	return &d
}

// StringValue returns the string value in v or an empty string if it's nil
func StringValue(v *string) string {
	if v != nil {
		return *v
	}
	return ""
}

// StringPtr returns a pointer to the provided string
func StringPtr(s string) *string {
	return &s
}

// FormatStringPtrWithDefault formats the string pointer value for output.
// If the pointer value is nil, the specified default is used
func FormatStringPtrWithDefault(v *string, def string) string {
	if v == nil {
		return def
	}
	return *v
}
