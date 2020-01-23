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

package aws

import "strings"

// Regions defines a map of supported EC2 regions to various attributes
// like machine image to use in any specific region.
var Regions = map[RegionName]RegionMapping{
	NVirginia:   {Image: "ami-366be821"},
	Ohio:        {Image: "ami-69045e0c"},
	NCalifornia: {Image: "ami-e4c78f84"},
	Oregon:      {Image: "ami-14b07274"},
	Ireland:     {Image: "ami-46591635"},
	Frankfurt:   {Image: "ami-3be11854"},
	Tokyo:       {Image: "ami-f6bd1a97"},
	Seoul:       {Image: "ami-1ff22671"},
	Singapore:   {Image: "ami-6662c405"},
	Sydney:      {Image: "ami-5e7e433d"},
	Mumbai:      {Image: "ami-dc6115b3"},
	SPaulo:      {Image: "ami-a578e5c9"},
	London:      {Image: "ami-5c32dc3b"},
	Paris:       {Image: "ami-6c16a711"},
	Canada:      {Image: "ami-c22cafa6"},
}

// RegionMapping defines the data an AWS EC2 region is mapped to
type RegionMapping struct {
	// Image is a reference to an Amazon Machine Image (AMI) in the specified region
	Image string
}

// RegionName defines an AWS EC2 region by name
type RegionName string

// SupportsInstanceType returns true if instances of the specified type can be provisioned
// in the specified region. The reason this function exists is AWS does not provide a sane
// way to check this via API.
//
// NOTE: Currently this function is aware only of certain regions/instance types that some
// of our customers care about and can be extended further as needed
func SupportsInstanceType(region, instanceType string) bool {
	switch RegionName(region) {
	case Seoul, Mumbai:
		if strings.HasPrefix(instanceType, "c3.") || strings.HasPrefix(instanceType, "m3.") {
			return false
		}
	}
	return true
}

// SupportedInstanceTypes returns a subset of the provided instance types list without the types
// that are not supported in the specified region
func SupportedInstanceTypes(region string, instanceTypes []string) []string {
	filtered := make([]string, 0, len(instanceTypes))
	for _, it := range instanceTypes {
		if SupportsInstanceType(region, it) {
			filtered = append(filtered, it)
		}
	}
	return filtered
}

const (
	NVirginia   RegionName = "us-east-1"
	Ohio        RegionName = "us-east-2"
	NCalifornia RegionName = "us-west-1"
	Oregon      RegionName = "us-west-2"
	Ireland     RegionName = "eu-west-1"
	London      RegionName = "eu-west-2"
	Paris       RegionName = "eu-west-3"
	Canada      RegionName = "ca-central-1"
	Beijing     RegionName = "cn-north-1"
	Frankfurt   RegionName = "eu-central-1"
	Tokyo       RegionName = "ap-northeast-1"
	Seoul       RegionName = "ap-northeast-2"
	OsakaLocal  RegionName = "ap-northeast-3"
	Singapore   RegionName = "ap-southeast-1"
	Sydney      RegionName = "ap-southeast-2"
	Mumbai      RegionName = "ap-south-1"
	SPaulo      RegionName = "sa-east-1"
)
