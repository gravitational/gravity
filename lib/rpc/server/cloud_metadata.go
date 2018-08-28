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
