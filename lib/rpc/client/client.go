package client

import (
	"context"
	"io"
	"time"

	"github.com/gravitational/gravity/lib/defaults"
	validationpb "github.com/gravitational/gravity/lib/network/validation/proto"
	pb "github.com/gravitational/gravity/lib/rpc/proto"
	"github.com/gravitational/gravity/lib/storage"

	"github.com/gravitational/satellite/agent/proto/agentpb"
	"github.com/gravitational/trace"

	"github.com/sirupsen/logrus"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
)

// Client is high level RPC agent interface
type Client interface {
	// Command executes the command specified with args remotely
	Command(ctx context.Context, log logrus.FieldLogger, out io.Writer, args ...string) error
	// GravityCommand executes the gravity command specified with args remotely
	GravityCommand(ctx context.Context, log logrus.FieldLogger, out io.Writer, args ...string) error
	// Validate validates the node against the specified manifest and profile.
	// Returns the list of failed probes
	Validate(ctx context.Context, req *validationpb.ValidateRequest) ([]*agentpb.Probe, error)
	// GetSystemInfo queries remote system information
	GetSystemInfo(context.Context) (storage.System, error)
	// GetRuntimeConfig returns agent's runtime configuration
	GetRuntimeConfig(context.Context) (*pb.RuntimeConfig, error)
	// GetCurrentTime returns agent's current time as UTC timestamp
	GetCurrentTime(context.Context) (*time.Time, error)
	// CheckPorts executes a network port test
	CheckPorts(context.Context, *validationpb.CheckPortsRequest) (*validationpb.CheckPortsResponse, error)
	// CheckBandwidth executes a network bandwidth test
	CheckBandwidth(context.Context, *validationpb.CheckBandwidthRequest) (*validationpb.CheckBandwidthResponse, error)
	// Shutdown requests remote agent to shut down
	Shutdown(context.Context) error
	// Close will close communication with remote agent
	Close() error
}

// Config defines configuration to connect to a remote RPC agent
type Config struct {
	// Credentials specifies connect credentials
	Credentials credentials.TransportCredentials
	// ServerAddr specifies the address of the remote node as host:port
	ServerAddr string
}

// New establishes connection to remote gRPC server
// note that if connection is unavailable, it will try to establish it
// until context provided expires
func New(ctx context.Context, config Config) (*client, error) {
	opts := append([]grpc.DialOption{
		grpc.WithBackoffMaxDelay(defaults.RPCAgentBackoffThreshold),
		grpc.WithBlock(),
		grpc.WithTransportCredentials(config.Credentials),
	})

	conn, err := grpc.DialContext(ctx, config.ServerAddr, opts...)
	if err != nil {
		return nil, trace.ConnectionProblem(err,
			"failed to establish connection to server at %v", config.ServerAddr)
	}

	return &client{
		agent:      pb.NewAgentClient(conn),
		discovery:  pb.NewDiscoveryClient(conn),
		validation: validationpb.NewValidationClient(conn),
		conn:       conn,
	}, nil
}

// NewFromConn creates a new client based on existing connection conn
func NewFromConn(conn *grpc.ClientConn) *client {
	return &client{
		agent:      pb.NewAgentClient(conn),
		discovery:  pb.NewDiscoveryClient(conn),
		validation: validationpb.NewValidationClient(conn),
		conn:       conn,
	}
}

// Close closes the underlying connection
func (c *client) Close() error {
	return c.conn.Close()
}

type client struct {
	agent      pb.AgentClient
	discovery  pb.DiscoveryClient
	validation validationpb.ValidationClient
	conn       *grpc.ClientConn
}
