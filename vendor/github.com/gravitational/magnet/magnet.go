package magnet

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/containerd/console"
	"github.com/gravitational/trace"
	"github.com/moby/buildkit/client"
	"github.com/moby/buildkit/util/progress/progressui"
	"github.com/opencontainers/go-digest"
)

var BuildLogDir string

func init() {
	BuildLogDir = filepath.Join("build/logs", time.Now().Format("20060102150405"))
}

type Magnet struct {
	Vertex *client.Vertex
	parent *Magnet
	status chan *client.SolveStatus

	statusLogger *SolveStatusLogger
}

var root *Magnet
var rootOnce sync.Once

func Root() *Magnet {
	const statusChanSize = 128

	rootOnce.Do(func() {
		now := time.Now()
		root = &Magnet{
			//status: make(chan *client.SolveStatus, statusChanSize),
			Vertex: &client.Vertex{
				Digest:    digest.FromString("root"),
				Name:      fmt.Sprint("Version: ", Version(), " Logs: ", BuildLogDir),
				Started:   &now,
				Completed: &now,
			},
			statusLogger: newSolveStatusLogger(BuildLogDir),
		}

		root.status = root.statusLogger.source
	})

	return root
}

// Shutdown indicates that the program is exiting, and we should shutdown the progressui
//  if it's currently running
func Shutdown() {
	if root != nil {
		// Hack: give progressui enough time to process any queues status updates
		time.Sleep(100 * time.Millisecond)
		close(root.status)
	}

	// Hack: give progressui enough time to update the display when shutting down
	time.Sleep(1000 * time.Millisecond)
}

func (m *Magnet) Clone(name string) *Magnet {
	started := time.Now()
	vertex := &client.Vertex{
		Digest:  digest.FromString(name),
		Name:    name,
		Started: &started,
	}

	status := &client.SolveStatus{
		Vertexes: []*client.Vertex{vertex},
	}

	m.root().status <- status

	return &Magnet{
		Vertex: vertex,
		parent: m,
	}
}

func InitOutput() {
	var c console.Console

	if os.Getenv("DEBIAN_FRONTEND") != "noninteractive" {
		if cn, err := console.ConsoleFromFile(os.Stderr); err == nil {
			c = cn
		}
	}

	go func() {
		err := progressui.DisplaySolveStatus(
			context.TODO(),
			Root().Vertex.Name,
			c,
			os.Stdout,
			Root().statusLogger.destination,
		)
		if err != nil {
			panic(trace.DebugReport(err))
		}
	}()
}

func (m *Magnet) root() *Magnet {
	root := m
	for root.parent != nil {
		root = root.parent
	}

	return root
}

// Complete marks the current task as complete.
func (m *Magnet) Complete(cached bool, err error) {
	now := time.Now()
	m.Vertex.Completed = &now
	m.Vertex.Cached = cached
	m.Vertex.Error = trace.DebugReport(err)

	m.root().status <- &client.SolveStatus{
		Vertexes: []*client.Vertex{
			m.Vertex,
		},
	}
}

// Printlnfallows writing log entries to the log output for the target.
func (m *Magnet) Println(args ...interface{}) {
	msg := fmt.Sprintln(args...)

	m.root().status <- &client.SolveStatus{
		Logs: []*client.VertexLog{
			{
				Vertex:    m.Vertex.Digest,
				Stream:    STDOUT,
				Data:      []byte(msg),
				Timestamp: time.Now(),
			},
		},
	}
}

// Printlnf allows writing log entries to the log output for the target.
func (m *Magnet) Printlnf(format string, args ...interface{}) {
	msg := fmt.Sprintf(format, args...)
	m.Println(msg)
}
