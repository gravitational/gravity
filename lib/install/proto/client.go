package installer

import (
	"context"
	"net"
	"path/filepath"
	"time"

	"github.com/gravitational/trace"
	log "github.com/sirupsen/logrus"
	grpc "google.golang.org/grpc"
)

// NewClient returns a new client using the specified state directory
// to look for socket file
func NewClient(ctx context.Context, stateDir string, logger log.FieldLogger) (AgentClient, error) {
	type result struct {
		*grpc.ClientConn
		error
	}
	resultC := make(chan result, 1)
	go func() {
		dialOptions := []grpc.DialOption{
			// Don't use TLS, as we communicate over domain sockers
			grpc.WithInsecure(),
			// Retry every second after failure
			grpc.WithBackoffMaxDelay(1 * time.Second),
			grpc.WithBlock(),
			grpc.WithContextDialer(func(ctx context.Context, addr string) (net.Conn, error) {
				conn, err := (&net.Dialer{}).DialContext(ctx, "unix", socketPath(stateDir))
				if err != nil {
					return nil, trace.Wrap(err)
				}
				return conn, nil
			}),
		}
		conn, err := grpc.Dial("unix:///installer.sock", dialOptions...)
		resultC <- result{ClientConn: conn, error: err}
	}()
	for {
		select {
		case result := <-resultC:
			if result.error != nil {
				return nil, trace.Wrap(result.error)
			}
			client := NewAgentClient(result.ClientConn)
			return client, nil
		case <-ctx.Done():
			logger.WithError(ctx.Err()).Warn("Failed to connect.")
			return nil, trace.Wrap(ctx.Err(), "interrupted")
		}
	}
}

func socketPath(stateDir string) (path string) {
	return filepath.Join(stateDir, "installer.sock")
}
