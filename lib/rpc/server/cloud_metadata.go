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

package server

import (
	"github.com/gravitational/gravity/lib/cloudprovider/aws"
	pb "github.com/gravitational/gravity/lib/rpc/proto"
	"github.com/gravitational/gravity/lib/schema"

	gcemeta "cloud.google.com/go/compute/metadata"
	"github.com/gravitational/trace"
)

// GetCloudMetadata fetches cloud metadata for the specified provider
func GetCloudMetadata(provider string) (*pb.CloudMetadata, error) {
	switch provider {
	case schema.ProviderAWS:
		return getAWSMetadata()
	case schema.ProviderGCE:
		return getGCEMetadata()
	}
	return nil, trace.BadParameter("unsupported cloud provider %q", provider)
}

func getAWSMetadata() (*pb.CloudMetadata, error) {
	instance, err := aws.NewLocalInstance()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return &pb.CloudMetadata{
		NodeName:     instance.NodeName,
		InstanceType: instance.Type,
		InstanceId:   instance.ID,
	}, nil
}

func getGCEMetadata() (*pb.CloudMetadata, error) {
	instanceID, err := gcemeta.InstanceID()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	instanceName, err := gcemeta.InstanceName()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	instanceType, err := gcemeta.Get("instance/machine-type")
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return &pb.CloudMetadata{
		NodeName:     instanceName,
		InstanceType: instanceType,
		InstanceId:   instanceID,
	}, nil
}
