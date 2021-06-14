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
	default:
		// No handling
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
	// NVirginia is the US east (North Virginia) region
	NVirginia RegionName = "us-east-1"
	// Ohio is the US east (Ohio) region
	Ohio RegionName = "us-east-2"
	// NCalifornia is the US west (North California) region
	NCalifornia RegionName = "us-west-1"
	// Oregon is the US west (Oregon) region
	Oregon RegionName = "us-west-2"
	// Ireland is the Europe (Ireland) region
	Ireland RegionName = "eu-west-1"
	// London is the Europe (London) region
	London RegionName = "eu-west-2"
	// Paris is the Europe (Paris) region
	Paris RegionName = "eu-west-3"
	// Canada is the Canada Central region
	Canada RegionName = "ca-central-1"
	// Beijing is the Beijing region
	Beijing RegionName = "cn-north-1"
	// Frankfurt is the Europe (Frankfurt) region
	Frankfurt RegionName = "eu-central-1"
	// Tokyo is the Asia Pacific (Tokyo) region
	Tokyo RegionName = "ap-northeast-1"
	// Seoul is the Asia Pacific (Seoul) region
	Seoul RegionName = "ap-northeast-2"
	// OsakaLocal is the Asia Pacific (Osaka) region
	OsakaLocal RegionName = "ap-northeast-3"
	// Singapore is the Asia Pacific (Singapore) region
	Singapore RegionName = "ap-southeast-1"
	// Sydney is the Asia Pacific (Sydney) region
	Sydney RegionName = "ap-southeast-2"
	// Mumbai is the Asia Pacific (Mumbai) region
	Mumbai RegionName = "ap-south-1"
	// SPaulo is the South America (São Paulo) region
	SPaulo RegionName = "sa-east-1"
)
