/*
Copyright 2019 Gravitational, Inc.

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

package catalog

import (
	"encoding/json"
	"fmt"
	"os"
	"text/tabwriter"
	"time"

	"github.com/ghodss/yaml"
	"github.com/gravitational/gravity/lib/constants"
	"github.com/gravitational/gravity/lib/hub"

	"github.com/gravitational/trace"
)

// Lister defines common interface for listing application and cluster images.
type Lister interface {
	//
	List(all bool) ([]ListItem, error)
}

// ListItem defines interface for a single list item.
type ListItem interface {
	GetName() string
	GetVersion() string
	GetType() string
	GetCreated() time.Time
	GetDescription() string
}

type hubLister struct{}

func NewLister() (*hubLister, error) {
	return &hubLister{}, nil
}

func (l *hubLister) List(all bool) (result []ListItem, err error) {
	hub, err := hub.New(hub.Config{})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	items, err := hub.List(all)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	for _, item := range items {
		result = append(result, item)
	}
	return result, nil
}

func List(lister Lister, all bool, format constants.Format) error {
	items, err := lister.List(all)
	if err != nil {
		return trace.Wrap(err)
	}
	switch format {
	case constants.EncodingText:
		w := new(tabwriter.Writer)
		w.Init(os.Stdout, 0, 8, 1, '\t', 0)
		fmt.Fprintf(w, "Name:Version\tType\tCreated (UTC)\tDescription\n")
		fmt.Fprintf(w, "------------\t----\t-------------\t-----------\n")
		for _, item := range items {
			fmt.Fprintf(w, "%v\t%v\t%v\t%v\n",
				formatName(item.GetName(), item.GetVersion()),
				item.GetType(),
				item.GetCreated().Format(constants.ShortDateFormat),
				formatDescription(item.GetDescription()))
		}
		w.Flush()
	case constants.EncodingJSON:
		bytes, err := json.MarshalIndent(items, "", "    ")
		if err != nil {
			return trace.Wrap(err)
		}
		fmt.Println(string(bytes))
	case constants.EncodingYAML:
		bytes, err := yaml.Marshal(items)
		if err != nil {
			return trace.Wrap(err)
		}
		fmt.Println(string(bytes))
	default:
		return trace.BadParameter("unknown output format %q, supported are: %v",
			format, constants.OutputFormats)
	}
	return nil
}

func formatName(name, version string) string {
	if name == constants.LegacyBaseImageName {
		name = constants.BaseImageName
	}
	return fmt.Sprintf("%v:%v", name, version)
}

func formatDescription(description string) string {
	if description == "" {
		return "-"
	}
	return description
}
