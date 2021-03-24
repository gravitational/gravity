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
	"sort"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/gravitational/gravity/lib/app"
	"github.com/gravitational/gravity/lib/constants"
	"github.com/gravitational/gravity/lib/defaults"
	"github.com/gravitational/gravity/lib/hub"
	"github.com/gravitational/gravity/lib/modules"
	"github.com/gravitational/gravity/lib/schema"

	"github.com/coreos/go-semver/semver"
	"github.com/fatih/color"
	"github.com/ghodss/yaml"
	"github.com/gravitational/trace"
)

// Lister defines common interface for listing application and cluster images.
type Lister interface {
	// List retrieves application and cluster images.
	List(all bool) (ListItems, error)
	// Hub returns the name of the Hub this lister talks to.
	Hub() string
}

// ListItem defines interface for a single list item.
type ListItem interface {
	// GetName returns the image name.
	GetName() string
	// GetVersion returns the image version.
	GetVersion() semver.Version
	// GetType returns the image type (application or cluster).
	GetType() string
	// GetCreated returns the image creation time.
	GetCreated() time.Time
	// GetDescription returns the image description.
	GetDescription() string
}

// listItem implements ListItem interface.
type listItem struct {
	// Name is the image name.
	Name string `json:"name"`
	// Version is the image version.
	Version semver.Version `json:"version"`
	// Created is the image creation timestamp.
	Created time.Time `json:"created"`
	// Type is the image type, application or cluster.
	Type string `json:"type"`
	// Description is the image description.
	Description string `json:"description"`
}

// NewListItemFromHubApp makes a list item from the hub application item.
func NewListItemFromHubApp(app hub.App) (*listItem, error) {
	semver, err := semver.NewVersion(app.Version)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return &listItem{
		Name:        convertName(app.Name),
		Version:     *semver,
		Created:     app.Created,
		Type:        app.Type,
		Description: app.Description,
	}, nil
}

// NewListItemFromApp makes a list item from the app service application.
func NewListItemFromApp(app app.Application) (*listItem, error) {
	semver, err := semver.NewVersion(app.Manifest.Metadata.ResourceVersion)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return &listItem{
		Name:        convertName(app.Manifest.Metadata.Name),
		Version:     *semver,
		Created:     app.PackageEnvelope.Created,
		Type:        app.Manifest.DescribeKind(),
		Description: app.Manifest.Metadata.Description,
	}, nil
}

// GetName returns the image name.
func (i listItem) GetName() string { return i.Name }

// GetVersion returns the image version.
func (i listItem) GetVersion() semver.Version { return i.Version }

// GetType returns the image type, application or cluster.
func (i listItem) GetType() string { return i.Type }

// GetCreated returns the image creation time.
func (i listItem) GetCreated() time.Time { return i.Created }

// GetDescription returns the image description.
func (i listItem) GetDescription() string { return i.Description }

// ListItems is a collection of application and cluster images.
type ListItems []ListItem

// Latest returns a list of items containing latest stable versions of
// application and cluster images from this list.
func (l ListItems) Latest() (result ListItems, err error) {
	m := make(map[string]ListItem)
	for _, item := range l {
		// Skip pre-releases.
		if item.GetVersion().PreRelease != "" {
			continue
		}
		if existing, ok := m[item.GetName()]; !ok {
			m[item.GetName()] = item
		} else {
			if existing.GetVersion().LessThan(item.GetVersion()) {
				m[item.GetName()] = item
			}
		}
	}
	for _, item := range m {
		result = append(result, item)
	}
	return result, nil
}

// Len implements sort.Interface.
func (l ListItems) Len() int {
	return len(l)
}

// Swap implements sort.Interface.
func (l ListItems) Swap(i, j int) {
	l[i], l[j] = l[j], l[i]
}

// Less implements sort.Interace.
//
// The items are sorted first by type (cluster images appear before application
// images), then by name (lexicographically) and finally by semantic version.
func (l ListItems) Less(i, j int) bool {
	if l[i].GetType() != l[j].GetType() {
		return l[i].GetType() == schema.KindCluster
	}
	if l[i].GetName() < l[j].GetName() {
		return true
	}
	// More recent versions should appear before older ones, so the "less"
	// logic is inverted here.
	if (l[i].GetName() == l[j].GetName()) && l[j].GetVersion().LessThan(l[i].GetVersion()) {
		return true
	}
	return false
}

type hubLister struct {
	hub hub.Hub
}

// NewLister returns a lister with S3 hub backend.
func NewLister() (*hubLister, error) {
	hub, err := hub.New(hub.Config{})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return &hubLister{hub: hub}, nil
}

// List returns application and cluster images from the hub.
func (l *hubLister) List(all bool) (result ListItems, err error) {
	items, err := l.hub.List(all)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	for _, item := range items {
		i, err := NewListItemFromHubApp(item)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		result = append(result, i)
	}
	return result, nil
}

// Hub returns the name of the open-source Hub.
func (l *hubLister) Hub() string {
	return defaults.HubBucket
}

// List uses the provided lister to obtain a list of application and
// cluster images and displays them in the specified format.
func List(lister Lister, all bool, format constants.Format) error {
	if !all {
		fmt.Print(color.YellowString("Displaying latest stable versions of application and cluster images in %v. Use --all flag to show all.\n\n",
			FormatHub(lister.Hub())))
	} else {
		fmt.Print(color.YellowString("Displaying all available application and cluster images in %v.\n\n",
			FormatHub(lister.Hub())))
	}
	items, err := lister.List(all)
	if err != nil {
		return trace.Wrap(err)
	}
	if !all {
		items, err = items.Latest()
		if err != nil {
			return trace.Wrap(err)
		}
	}
	sort.Sort(items)
	switch format {
	case constants.EncodingText:
		w := new(tabwriter.Writer)
		w.Init(os.Stdout, 0, 8, 1, '\t', 0)
		fmt.Fprintf(w, "Name:Version\tImage Type\tCreated (UTC)\tDescription\n")
		fmt.Fprintf(w, "------------\t----------\t-------------\t-----------\n")
		for _, item := range items {
			fmt.Fprintf(w, "%v:%v\t%v\t%v\t%v\n",
				item.GetName(), item.GetVersion().String(),
				item.GetType(),
				item.GetCreated().Format(constants.ShortDateFormat),
				formatDescription(item.GetDescription()))
		}
		w.Flush()
	case constants.EncodingShort:
                sb := new(strings.Builder)
                for _, item := range items {
                        sb.WriteString(item.GetName() + ":" + item.GetVersion().String() + "\n")
                }
                fmt.Print(sb.String())
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

// FormatHub formats the message about selected Hub for the user.
func FormatHub(hub string) string {
	if hub == modules.Get().TeleRepository() {
		return "the default Gravitational Hub"
	}
	return hub
}

func convertName(name string) string {
	switch name {
	case constants.LegacyBaseImageName:
		return constants.BaseImageName
	case constants.LegacyHubImageName:
		return constants.HubImageName
	}
	return name
}

func formatDescription(description string) string {
	if description == "" {
		return "-"
	}
	return strings.TrimSpace(description)
}
