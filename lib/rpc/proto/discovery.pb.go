// Code generated by protoc-gen-gogo. DO NOT EDIT.
// source: discovery.proto

package proto

import (
	context "context"
	fmt "fmt"
	_ "github.com/gogo/protobuf/gogoproto"
	proto "github.com/gogo/protobuf/proto"
	types "github.com/gogo/protobuf/types"
	grpc "google.golang.org/grpc"
	codes "google.golang.org/grpc/codes"
	status "google.golang.org/grpc/status"
	math "math"
)

// Reference imports to suppress errors if they are not otherwise used.
var _ = proto.Marshal
var _ = fmt.Errorf
var _ = math.Inf

// This is a compile-time assertion to ensure that this generated file
// is compatible with the proto package it is being compiled against.
// A compilation error at this line likely means your copy of the
// proto package needs to be updated.
const _ = proto.GoGoProtoPackageIsVersion3 // please upgrade the proto package

// Version represents a gravity version
type Version struct {
	// Edition is the gravity edition, e.g. open-source
	Edition string `protobuf:"bytes,1,opt,name=edition,proto3" json:"edition,omitempty"`
	// Version is the gravity semantic version
	Version string `protobuf:"bytes,2,opt,name=version,proto3" json:"version,omitempty"`
	// GitCommit is the git commit hash
	GitCommit string `protobuf:"bytes,3,opt,name=git_commit,json=gitCommit,proto3" json:"git_commit,omitempty"`
	// Helm is the built-in Helm version
	Helm                 string   `protobuf:"bytes,4,opt,name=helm,proto3" json:"helm,omitempty"`
	XXX_NoUnkeyedLiteral struct{} `json:"-"`
	XXX_unrecognized     []byte   `json:"-"`
	XXX_sizecache        int32    `json:"-"`
}

func (m *Version) Reset()      { *m = Version{} }
func (*Version) ProtoMessage() {}
func (*Version) Descriptor() ([]byte, []int) {
	return fileDescriptor_1e7ff60feb39c8d0, []int{0}
}
func (m *Version) XXX_Unmarshal(b []byte) error {
	return xxx_messageInfo_Version.Unmarshal(m, b)
}
func (m *Version) XXX_Marshal(b []byte, deterministic bool) ([]byte, error) {
	return xxx_messageInfo_Version.Marshal(b, m, deterministic)
}
func (m *Version) XXX_Merge(src proto.Message) {
	xxx_messageInfo_Version.Merge(m, src)
}
func (m *Version) XXX_Size() int {
	return xxx_messageInfo_Version.Size(m)
}
func (m *Version) XXX_DiscardUnknown() {
	xxx_messageInfo_Version.DiscardUnknown(m)
}

var xxx_messageInfo_Version proto.InternalMessageInfo

func (m *Version) GetEdition() string {
	if m != nil {
		return m.Edition
	}
	return ""
}

func (m *Version) GetVersion() string {
	if m != nil {
		return m.Version
	}
	return ""
}

func (m *Version) GetGitCommit() string {
	if m != nil {
		return m.GitCommit
	}
	return ""
}

func (m *Version) GetHelm() string {
	if m != nil {
		return m.Helm
	}
	return ""
}

// SystemInfo groups attributes that describe a system
type SystemInfo struct {
	// Payload is the opaque system information payload.
	Payload              []byte   `protobuf:"bytes,1,opt,name=payload,proto3" json:"payload,omitempty"`
	XXX_NoUnkeyedLiteral struct{} `json:"-"`
	XXX_unrecognized     []byte   `json:"-"`
	XXX_sizecache        int32    `json:"-"`
}

func (m *SystemInfo) Reset()         { *m = SystemInfo{} }
func (m *SystemInfo) String() string { return proto.CompactTextString(m) }
func (*SystemInfo) ProtoMessage()    {}
func (*SystemInfo) Descriptor() ([]byte, []int) {
	return fileDescriptor_1e7ff60feb39c8d0, []int{1}
}
func (m *SystemInfo) XXX_Unmarshal(b []byte) error {
	return xxx_messageInfo_SystemInfo.Unmarshal(m, b)
}
func (m *SystemInfo) XXX_Marshal(b []byte, deterministic bool) ([]byte, error) {
	return xxx_messageInfo_SystemInfo.Marshal(b, m, deterministic)
}
func (m *SystemInfo) XXX_Merge(src proto.Message) {
	xxx_messageInfo_SystemInfo.Merge(m, src)
}
func (m *SystemInfo) XXX_Size() int {
	return xxx_messageInfo_SystemInfo.Size(m)
}
func (m *SystemInfo) XXX_DiscardUnknown() {
	xxx_messageInfo_SystemInfo.DiscardUnknown(m)
}

var xxx_messageInfo_SystemInfo proto.InternalMessageInfo

func (m *SystemInfo) GetPayload() []byte {
	if m != nil {
		return m.Payload
	}
	return nil
}

// RuntimeConfig decribes agent's runtime configuration
type RuntimeConfig struct {
	// Role specifies the agent's role
	Role string `protobuf:"bytes,1,opt,name=role,proto3" json:"role,omitempty"`
	// AdvertiseAddr specifies the address the agent is listening on
	AdvertiseAddr string `protobuf:"bytes,2,opt,name=advertise_addr,json=advertiseAddr,proto3" json:"advertise_addr,omitempty"`
	// SystemDevice defines the device to use to setup telekube state directory
	SystemDevice string `protobuf:"bytes,4,opt,name=system_device,json=systemDevice,proto3" json:"system_device,omitempty"`
	// Mounts lists mount overrides
	Mounts []*Mount `protobuf:"bytes,5,rep,name=mounts,proto3" json:"mounts,omitempty"`
	// StateDir specifies the location of the telekube system state directory
	StateDir string `protobuf:"bytes,6,opt,name=state_dir,json=stateDir,proto3" json:"state_dir,omitempty"`
	// TODO: TempDir specifies the location of the temporary directory. It is part of the runtime
	// configuration as it can potentially become local to system directory - independent of OS
	TempDir string `protobuf:"bytes,7,opt,name=temp_dir,json=tempDir,proto3" json:"temp_dir,omitempty"`
	// Token authorizes the peer to join an operation.
	Token string `protobuf:"bytes,8,opt,name=token,proto3" json:"token,omitempty"`
	// CloudMetadata describes the instance from the cloud environment.
	// Optional, only set when running on a supported cloud
	CloudMetadata *CloudMetadata `protobuf:"bytes,9,opt,name=cloud_metadata,json=cloudMetadata,proto3" json:"cloud_metadata,omitempty"`
	// KeyValues specifies additional runtime configuration parameters
	// as key=value pairs
	KeyValues map[string]string `protobuf:"bytes,10,rep,name=key_values,json=keyValues,proto3" json:"key_values,omitempty" protobuf_key:"bytes,1,opt,name=key,proto3" protobuf_val:"bytes,2,opt,name=value,proto3"`
	// SELinux indicates that the node has SELinux support on
	SELinux              bool     `protobuf:"varint,11,opt,name=selinux,proto3" json:"selinux,omitempty"`
	XXX_NoUnkeyedLiteral struct{} `json:"-"`
	XXX_unrecognized     []byte   `json:"-"`
	XXX_sizecache        int32    `json:"-"`
}

func (m *RuntimeConfig) Reset()      { *m = RuntimeConfig{} }
func (*RuntimeConfig) ProtoMessage() {}
func (*RuntimeConfig) Descriptor() ([]byte, []int) {
	return fileDescriptor_1e7ff60feb39c8d0, []int{2}
}
func (m *RuntimeConfig) XXX_Unmarshal(b []byte) error {
	return xxx_messageInfo_RuntimeConfig.Unmarshal(m, b)
}
func (m *RuntimeConfig) XXX_Marshal(b []byte, deterministic bool) ([]byte, error) {
	return xxx_messageInfo_RuntimeConfig.Marshal(b, m, deterministic)
}
func (m *RuntimeConfig) XXX_Merge(src proto.Message) {
	xxx_messageInfo_RuntimeConfig.Merge(m, src)
}
func (m *RuntimeConfig) XXX_Size() int {
	return xxx_messageInfo_RuntimeConfig.Size(m)
}
func (m *RuntimeConfig) XXX_DiscardUnknown() {
	xxx_messageInfo_RuntimeConfig.DiscardUnknown(m)
}

var xxx_messageInfo_RuntimeConfig proto.InternalMessageInfo

func (m *RuntimeConfig) GetRole() string {
	if m != nil {
		return m.Role
	}
	return ""
}

func (m *RuntimeConfig) GetAdvertiseAddr() string {
	if m != nil {
		return m.AdvertiseAddr
	}
	return ""
}

func (m *RuntimeConfig) GetSystemDevice() string {
	if m != nil {
		return m.SystemDevice
	}
	return ""
}

func (m *RuntimeConfig) GetMounts() []*Mount {
	if m != nil {
		return m.Mounts
	}
	return nil
}

func (m *RuntimeConfig) GetStateDir() string {
	if m != nil {
		return m.StateDir
	}
	return ""
}

func (m *RuntimeConfig) GetTempDir() string {
	if m != nil {
		return m.TempDir
	}
	return ""
}

func (m *RuntimeConfig) GetToken() string {
	if m != nil {
		return m.Token
	}
	return ""
}

func (m *RuntimeConfig) GetCloudMetadata() *CloudMetadata {
	if m != nil {
		return m.CloudMetadata
	}
	return nil
}

func (m *RuntimeConfig) GetKeyValues() map[string]string {
	if m != nil {
		return m.KeyValues
	}
	return nil
}

func (m *RuntimeConfig) GetSELinux() bool {
	if m != nil {
		return m.SELinux
	}
	return false
}

// Device is a system I/O device
type Device struct {
	// Name identifies the device
	Name                 string   `protobuf:"bytes,1,opt,name=name,proto3" json:"name,omitempty"`
	XXX_NoUnkeyedLiteral struct{} `json:"-"`
	XXX_unrecognized     []byte   `json:"-"`
	XXX_sizecache        int32    `json:"-"`
}

func (m *Device) Reset()         { *m = Device{} }
func (m *Device) String() string { return proto.CompactTextString(m) }
func (*Device) ProtoMessage()    {}
func (*Device) Descriptor() ([]byte, []int) {
	return fileDescriptor_1e7ff60feb39c8d0, []int{3}
}
func (m *Device) XXX_Unmarshal(b []byte) error {
	return xxx_messageInfo_Device.Unmarshal(m, b)
}
func (m *Device) XXX_Marshal(b []byte, deterministic bool) ([]byte, error) {
	return xxx_messageInfo_Device.Marshal(b, m, deterministic)
}
func (m *Device) XXX_Merge(src proto.Message) {
	xxx_messageInfo_Device.Merge(m, src)
}
func (m *Device) XXX_Size() int {
	return xxx_messageInfo_Device.Size(m)
}
func (m *Device) XXX_DiscardUnknown() {
	xxx_messageInfo_Device.DiscardUnknown(m)
}

var xxx_messageInfo_Device proto.InternalMessageInfo

func (m *Device) GetName() string {
	if m != nil {
		return m.Name
	}
	return ""
}

// Mount is mount point
type Mount struct {
	// Name identifies the mount point
	Name string `protobuf:"bytes,1,opt,name=name,proto3" json:"name,omitempty"`
	// Source specifies the source directory
	Source               string   `protobuf:"bytes,2,opt,name=source,proto3" json:"source,omitempty"`
	XXX_NoUnkeyedLiteral struct{} `json:"-"`
	XXX_unrecognized     []byte   `json:"-"`
	XXX_sizecache        int32    `json:"-"`
}

func (m *Mount) Reset()      { *m = Mount{} }
func (*Mount) ProtoMessage() {}
func (*Mount) Descriptor() ([]byte, []int) {
	return fileDescriptor_1e7ff60feb39c8d0, []int{4}
}
func (m *Mount) XXX_Unmarshal(b []byte) error {
	return xxx_messageInfo_Mount.Unmarshal(m, b)
}
func (m *Mount) XXX_Marshal(b []byte, deterministic bool) ([]byte, error) {
	return xxx_messageInfo_Mount.Marshal(b, m, deterministic)
}
func (m *Mount) XXX_Merge(src proto.Message) {
	xxx_messageInfo_Mount.Merge(m, src)
}
func (m *Mount) XXX_Size() int {
	return xxx_messageInfo_Mount.Size(m)
}
func (m *Mount) XXX_DiscardUnknown() {
	xxx_messageInfo_Mount.DiscardUnknown(m)
}

var xxx_messageInfo_Mount proto.InternalMessageInfo

func (m *Mount) GetName() string {
	if m != nil {
		return m.Name
	}
	return ""
}

func (m *Mount) GetSource() string {
	if m != nil {
		return m.Source
	}
	return ""
}

// CloudMetadata describes the instance in the cloud
type CloudMetadata struct {
	// NodeName specifies the optional node name override.
	// Can be specified either on command line or derived from cloud environment.
	NodeName string `protobuf:"bytes,1,opt,name=node_name,json=nodeName,proto3" json:"node_name,omitempty"`
	// InstanceType specifies the type of the instance running
	InstanceType string `protobuf:"bytes,2,opt,name=instance_type,json=instanceType,proto3" json:"instance_type,omitempty"`
	// InstanceID provides the cloud-specific instance ID
	InstanceId           string   `protobuf:"bytes,3,opt,name=instance_id,json=instanceId,proto3" json:"instance_id,omitempty"`
	XXX_NoUnkeyedLiteral struct{} `json:"-"`
	XXX_unrecognized     []byte   `json:"-"`
	XXX_sizecache        int32    `json:"-"`
}

func (m *CloudMetadata) Reset()      { *m = CloudMetadata{} }
func (*CloudMetadata) ProtoMessage() {}
func (*CloudMetadata) Descriptor() ([]byte, []int) {
	return fileDescriptor_1e7ff60feb39c8d0, []int{5}
}
func (m *CloudMetadata) XXX_Unmarshal(b []byte) error {
	return xxx_messageInfo_CloudMetadata.Unmarshal(m, b)
}
func (m *CloudMetadata) XXX_Marshal(b []byte, deterministic bool) ([]byte, error) {
	return xxx_messageInfo_CloudMetadata.Marshal(b, m, deterministic)
}
func (m *CloudMetadata) XXX_Merge(src proto.Message) {
	xxx_messageInfo_CloudMetadata.Merge(m, src)
}
func (m *CloudMetadata) XXX_Size() int {
	return xxx_messageInfo_CloudMetadata.Size(m)
}
func (m *CloudMetadata) XXX_DiscardUnknown() {
	xxx_messageInfo_CloudMetadata.DiscardUnknown(m)
}

var xxx_messageInfo_CloudMetadata proto.InternalMessageInfo

func (m *CloudMetadata) GetNodeName() string {
	if m != nil {
		return m.NodeName
	}
	return ""
}

func (m *CloudMetadata) GetInstanceType() string {
	if m != nil {
		return m.InstanceType
	}
	return ""
}

func (m *CloudMetadata) GetInstanceId() string {
	if m != nil {
		return m.InstanceId
	}
	return ""
}

func init() {
	proto.RegisterType((*Version)(nil), "proto.Version")
	proto.RegisterType((*SystemInfo)(nil), "proto.SystemInfo")
	proto.RegisterType((*RuntimeConfig)(nil), "proto.RuntimeConfig")
	proto.RegisterMapType((map[string]string)(nil), "proto.RuntimeConfig.KeyValuesEntry")
	proto.RegisterType((*Device)(nil), "proto.Device")
	proto.RegisterType((*Mount)(nil), "proto.Mount")
	proto.RegisterType((*CloudMetadata)(nil), "proto.CloudMetadata")
}

func init() { proto.RegisterFile("discovery.proto", fileDescriptor_1e7ff60feb39c8d0) }

var fileDescriptor_1e7ff60feb39c8d0 = []byte{
	// 635 bytes of a gzipped FileDescriptorProto
	0x1f, 0x8b, 0x08, 0x00, 0x00, 0x00, 0x00, 0x00, 0x02, 0xff, 0x74, 0x54, 0xcd, 0x6e, 0xd3, 0x40,
	0x10, 0x26, 0xcd, 0xff, 0xe4, 0x87, 0xb2, 0xaa, 0x2a, 0x93, 0x82, 0x12, 0xa5, 0x14, 0xe5, 0x94,
	0x4a, 0x85, 0x03, 0x14, 0x0e, 0xd0, 0xa4, 0x8a, 0x0a, 0x94, 0x83, 0x5b, 0xf5, 0x6a, 0xb9, 0xde,
	0xa9, 0x59, 0xc5, 0xde, 0xb5, 0xd6, 0xeb, 0xa8, 0x7e, 0x00, 0xde, 0x81, 0x23, 0x2f, 0xc5, 0x95,
	0x03, 0x4f, 0x82, 0x76, 0xbd, 0xae, 0x12, 0x68, 0x4f, 0x9e, 0xef, 0xfb, 0x66, 0x67, 0xc6, 0x33,
	0x3b, 0x0b, 0x8f, 0x29, 0x4b, 0x03, 0xb1, 0x42, 0x99, 0x4f, 0x13, 0x29, 0x94, 0x20, 0x75, 0xf3,
	0x19, 0xec, 0x85, 0x42, 0x84, 0x11, 0x1e, 0x1a, 0x74, 0x9d, 0xdd, 0x1c, 0x62, 0x9c, 0x28, 0xeb,
	0x33, 0x18, 0xfe, 0x2b, 0x2a, 0x16, 0x63, 0xaa, 0xfc, 0x38, 0xb1, 0x0e, 0x10, 0x8a, 0x50, 0x14,
	0xf6, 0x78, 0x05, 0xcd, 0x2b, 0x94, 0x29, 0x13, 0x9c, 0x38, 0xd0, 0x44, 0xca, 0x14, 0x13, 0xdc,
	0xa9, 0x8c, 0x2a, 0x93, 0xb6, 0x5b, 0x42, 0xad, 0xac, 0x0a, 0x27, 0x67, 0xab, 0x50, 0x2c, 0x24,
	0xcf, 0x01, 0x42, 0xa6, 0xbc, 0x40, 0xc4, 0x31, 0x53, 0x4e, 0xd5, 0x88, 0xed, 0x90, 0xa9, 0x99,
	0x21, 0x08, 0x81, 0xda, 0x37, 0x8c, 0x62, 0xa7, 0x66, 0x04, 0x63, 0x1f, 0xd7, 0x7e, 0xfc, 0x1c,
	0x3e, 0x1a, 0xbf, 0x04, 0xb8, 0xc8, 0x53, 0x85, 0xf1, 0x19, 0xbf, 0x11, 0x3a, 0x41, 0xe2, 0xe7,
	0x91, 0xf0, 0xa9, 0x49, 0xdd, 0x75, 0x4b, 0x38, 0xfe, 0x55, 0x85, 0x9e, 0x9b, 0x71, 0xfd, 0x0b,
	0x33, 0xc1, 0x6f, 0x58, 0xa8, 0x63, 0x4a, 0x11, 0xa1, 0xad, 0xd1, 0xd8, 0xe4, 0x00, 0xfa, 0x3e,
	0x5d, 0xa1, 0x54, 0x2c, 0x45, 0xcf, 0xa7, 0x54, 0xda, 0x3a, 0x7b, 0x77, 0xec, 0x47, 0x4a, 0x25,
	0xd9, 0x87, 0x5e, 0x6a, 0x92, 0x7a, 0x14, 0x57, 0x2c, 0x40, 0x5b, 0x57, 0xb7, 0x20, 0xe7, 0x86,
	0x23, 0x2f, 0xa0, 0x11, 0x8b, 0x8c, 0xab, 0xd4, 0xa9, 0x8f, 0xaa, 0x93, 0xce, 0x51, 0xb7, 0xe8,
	0xd4, 0xf4, 0x5c, 0x93, 0xae, 0xd5, 0xc8, 0x1e, 0xb4, 0x53, 0xe5, 0x2b, 0xf4, 0x28, 0x93, 0x4e,
	0xc3, 0x84, 0x69, 0x19, 0x62, 0xce, 0x24, 0x79, 0x0a, 0x2d, 0x85, 0x71, 0x62, 0xb4, 0x66, 0xd1,
	0x30, 0x8d, 0xb5, 0xb4, 0x03, 0x75, 0x25, 0x96, 0xc8, 0x9d, 0x96, 0xe1, 0x0b, 0x40, 0xde, 0x41,
	0x3f, 0x88, 0x44, 0x46, 0xbd, 0x18, 0x95, 0x4f, 0x7d, 0xe5, 0x3b, 0xed, 0x51, 0x65, 0xd2, 0x39,
	0xda, 0xb1, 0xb9, 0x67, 0x5a, 0x3c, 0xb7, 0x9a, 0xdb, 0x0b, 0xd6, 0x21, 0x39, 0x01, 0x58, 0x62,
	0xee, 0xad, 0xfc, 0x28, 0xc3, 0xd4, 0x01, 0x53, 0xf4, 0xbe, 0x3d, 0xb8, 0xd1, 0xba, 0xe9, 0x67,
	0xcc, 0xaf, 0x8c, 0xd7, 0x29, 0x57, 0x32, 0x77, 0xdb, 0xcb, 0x12, 0x93, 0x03, 0x68, 0xa6, 0x18,
	0x31, 0x9e, 0xdd, 0x3a, 0x9d, 0x51, 0x65, 0xd2, 0x3a, 0xe9, 0xfc, 0xf9, 0x3d, 0x6c, 0x5e, 0x9c,
	0x7e, 0xd1, 0x94, 0x5b, 0x6a, 0x83, 0xf7, 0xd0, 0xdf, 0x8c, 0x41, 0xb6, 0xa1, 0xba, 0xc4, 0xdc,
	0x0e, 0x43, 0x9b, 0xfa, 0x0f, 0x4d, 0x29, 0x76, 0x04, 0x05, 0x38, 0xde, 0x7a, 0x53, 0x29, 0x26,
	0xff, 0xa9, 0xd6, 0xaa, 0x6e, 0xd7, 0xc6, 0xcf, 0xa0, 0x61, 0xfb, 0x4d, 0xa0, 0xc6, 0xfd, 0xf8,
	0x6e, 0x9e, 0xda, 0x1e, 0xbf, 0x85, 0xba, 0x69, 0xf7, 0x7d, 0x22, 0xd9, 0x85, 0x46, 0x2a, 0x32,
	0x19, 0x94, 0x19, 0x2c, 0xb2, 0x17, 0xeb, 0x16, 0x7a, 0x1b, 0xdd, 0xd2, 0x93, 0xe2, 0x82, 0xa2,
	0xb7, 0x16, 0xa7, 0xa5, 0x89, 0xaf, 0x3a, 0xd6, 0x3e, 0xf4, 0x18, 0x4f, 0x95, 0xcf, 0x03, 0xf4,
	0x54, 0x9e, 0x94, 0x21, 0xbb, 0x25, 0x79, 0x99, 0x27, 0x48, 0x86, 0xd0, 0xb9, 0x73, 0x62, 0xd4,
	0xde, 0x72, 0x28, 0xa9, 0x33, 0x5a, 0x64, 0x3e, 0xfa, 0xbe, 0x05, 0xed, 0x79, 0xb9, 0xaf, 0xe4,
	0x03, 0x6c, 0x2f, 0x50, 0x6d, 0x5e, 0xdd, 0xdd, 0x69, 0xb1, 0x9a, 0xd3, 0x72, 0x35, 0xa7, 0xa7,
	0x7a, 0x6f, 0x07, 0x3b, 0xf7, 0x4d, 0x8b, 0x1c, 0x43, 0x6f, 0x81, 0x6a, 0x6d, 0x4b, 0x1e, 0x3a,
	0xfe, 0xc4, 0x1e, 0x5f, 0x73, 0x9d, 0x43, 0x7f, 0x81, 0x6a, 0x96, 0x49, 0x89, 0x5c, 0x5d, 0x32,
	0xdd, 0xb5, 0x07, 0x0e, 0x0f, 0xfe, 0xe3, 0x2f, 0xcb, 0xe7, 0x82, 0xbc, 0x06, 0x58, 0xa0, 0x2a,
	0xdf, 0x87, 0x87, 0x22, 0xf4, 0x6d, 0x7a, 0xeb, 0x77, 0xdd, 0x30, 0xf0, 0xd5, 0xdf, 0x00, 0x00,
	0x00, 0xff, 0xff, 0xce, 0xe0, 0xf7, 0x71, 0xbd, 0x04, 0x00, 0x00,
}

// Reference imports to suppress errors if they are not otherwise used.
var _ context.Context
var _ grpc.ClientConn

// This is a compile-time assertion to ensure that this generated file
// is compatible with the grpc package it is being compiled against.
const _ = grpc.SupportPackageIsVersion4

// DiscoveryClient is the client API for Discovery service.
//
// For semantics around ctx use and closing/ending streaming RPCs, please refer to https://godoc.org/google.golang.org/grpc#ClientConn.NewStream.
type DiscoveryClient interface {
	// GetRuntimeConfig returns the agent's runtime configuration
	GetRuntimeConfig(ctx context.Context, in *types.Empty, opts ...grpc.CallOption) (*RuntimeConfig, error)
	// GetSystemInfo queries system information on the host the agent is running on
	GetSystemInfo(ctx context.Context, in *types.Empty, opts ...grpc.CallOption) (*SystemInfo, error)
	// GetCurrentTime returns the node's current time as UTC timestamp
	GetCurrentTime(ctx context.Context, in *types.Empty, opts ...grpc.CallOption) (*types.Timestamp, error)
	// GetVersion returns the agent's version information.
	GetVersion(ctx context.Context, in *types.Empty, opts ...grpc.CallOption) (*Version, error)
}

type discoveryClient struct {
	cc *grpc.ClientConn
}

func NewDiscoveryClient(cc *grpc.ClientConn) DiscoveryClient {
	return &discoveryClient{cc}
}

func (c *discoveryClient) GetRuntimeConfig(ctx context.Context, in *types.Empty, opts ...grpc.CallOption) (*RuntimeConfig, error) {
	out := new(RuntimeConfig)
	err := c.cc.Invoke(ctx, "/proto.Discovery/GetRuntimeConfig", in, out, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *discoveryClient) GetSystemInfo(ctx context.Context, in *types.Empty, opts ...grpc.CallOption) (*SystemInfo, error) {
	out := new(SystemInfo)
	err := c.cc.Invoke(ctx, "/proto.Discovery/GetSystemInfo", in, out, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *discoveryClient) GetCurrentTime(ctx context.Context, in *types.Empty, opts ...grpc.CallOption) (*types.Timestamp, error) {
	out := new(types.Timestamp)
	err := c.cc.Invoke(ctx, "/proto.Discovery/GetCurrentTime", in, out, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *discoveryClient) GetVersion(ctx context.Context, in *types.Empty, opts ...grpc.CallOption) (*Version, error) {
	out := new(Version)
	err := c.cc.Invoke(ctx, "/proto.Discovery/GetVersion", in, out, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

// DiscoveryServer is the server API for Discovery service.
type DiscoveryServer interface {
	// GetRuntimeConfig returns the agent's runtime configuration
	GetRuntimeConfig(context.Context, *types.Empty) (*RuntimeConfig, error)
	// GetSystemInfo queries system information on the host the agent is running on
	GetSystemInfo(context.Context, *types.Empty) (*SystemInfo, error)
	// GetCurrentTime returns the node's current time as UTC timestamp
	GetCurrentTime(context.Context, *types.Empty) (*types.Timestamp, error)
	// GetVersion returns the agent's version information.
	GetVersion(context.Context, *types.Empty) (*Version, error)
}

// UnimplementedDiscoveryServer can be embedded to have forward compatible implementations.
type UnimplementedDiscoveryServer struct {
}

func (*UnimplementedDiscoveryServer) GetRuntimeConfig(ctx context.Context, req *types.Empty) (*RuntimeConfig, error) {
	return nil, status.Errorf(codes.Unimplemented, "method GetRuntimeConfig not implemented")
}
func (*UnimplementedDiscoveryServer) GetSystemInfo(ctx context.Context, req *types.Empty) (*SystemInfo, error) {
	return nil, status.Errorf(codes.Unimplemented, "method GetSystemInfo not implemented")
}
func (*UnimplementedDiscoveryServer) GetCurrentTime(ctx context.Context, req *types.Empty) (*types.Timestamp, error) {
	return nil, status.Errorf(codes.Unimplemented, "method GetCurrentTime not implemented")
}
func (*UnimplementedDiscoveryServer) GetVersion(ctx context.Context, req *types.Empty) (*Version, error) {
	return nil, status.Errorf(codes.Unimplemented, "method GetVersion not implemented")
}

func RegisterDiscoveryServer(s *grpc.Server, srv DiscoveryServer) {
	s.RegisterService(&_Discovery_serviceDesc, srv)
}

func _Discovery_GetRuntimeConfig_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(types.Empty)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(DiscoveryServer).GetRuntimeConfig(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: "/proto.Discovery/GetRuntimeConfig",
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(DiscoveryServer).GetRuntimeConfig(ctx, req.(*types.Empty))
	}
	return interceptor(ctx, in, info, handler)
}

func _Discovery_GetSystemInfo_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(types.Empty)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(DiscoveryServer).GetSystemInfo(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: "/proto.Discovery/GetSystemInfo",
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(DiscoveryServer).GetSystemInfo(ctx, req.(*types.Empty))
	}
	return interceptor(ctx, in, info, handler)
}

func _Discovery_GetCurrentTime_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(types.Empty)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(DiscoveryServer).GetCurrentTime(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: "/proto.Discovery/GetCurrentTime",
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(DiscoveryServer).GetCurrentTime(ctx, req.(*types.Empty))
	}
	return interceptor(ctx, in, info, handler)
}

func _Discovery_GetVersion_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(types.Empty)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(DiscoveryServer).GetVersion(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: "/proto.Discovery/GetVersion",
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(DiscoveryServer).GetVersion(ctx, req.(*types.Empty))
	}
	return interceptor(ctx, in, info, handler)
}

var _Discovery_serviceDesc = grpc.ServiceDesc{
	ServiceName: "proto.Discovery",
	HandlerType: (*DiscoveryServer)(nil),
	Methods: []grpc.MethodDesc{
		{
			MethodName: "GetRuntimeConfig",
			Handler:    _Discovery_GetRuntimeConfig_Handler,
		},
		{
			MethodName: "GetSystemInfo",
			Handler:    _Discovery_GetSystemInfo_Handler,
		},
		{
			MethodName: "GetCurrentTime",
			Handler:    _Discovery_GetCurrentTime_Handler,
		},
		{
			MethodName: "GetVersion",
			Handler:    _Discovery_GetVersion_Handler,
		},
	},
	Streams:  []grpc.StreamDesc{},
	Metadata: "discovery.proto",
}
