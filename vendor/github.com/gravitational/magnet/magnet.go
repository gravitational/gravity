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

	//"github.com/moby/buildkit/util/progress/progressui"

	"github.com/gravitational/magnet/pkg/progressui"

	"github.com/opencontainers/go-digest"
)

type Config struct {
	LogDir   string
	Version  string
	BuildDir string

	PrintConfig bool
}

func (c *Config) CheckAndSetDefaults() {
	if c.Version == "" {
		c.Version = DefaultVersion()
	}

	if c.LogDir == "" {
		c.LogDir = DefaultLogDir()
	}

	if c.BuildDir == "" {
		c.BuildDir = DefaultBuildDir(c.Version)
	}
}

func (m *Magnet) printHeader() {
	fmt.Printf("Logs:    %v (%v)\n", m.statusLogger.dirLink(), m.statusLogger.dirReal())

	if m.Version != "" {
		fmt.Println("Version: ", m.Version)
	}

	if m.BuildDir != "" {
		fmt.Println("Build:   ", m.BuildDir)
	}
}

type Magnet struct {
	Config

	Vertex *progressui.Vertex
	parent *Magnet
	status chan *progressui.SolveStatus

	statusLogger *SolveStatusLogger

	cached bool
}

var root *Magnet
var rootOnce sync.Once
var outputOnce sync.Once

// Root creates a root vertex for executing and capturing status of each build target.
func Root(c Config) *Magnet {
	const statusChanSize = 128

	rootOnce.Do(func() {
		c.CheckAndSetDefaults()

		now := time.Now()
		root = &Magnet{
			Config: c,
			Vertex: &progressui.Vertex{
				Digest:    digest.FromString("root"),
				Started:   &now,
				Completed: &now,
			},
			statusLogger: newSolveStatusLogger(c.LogDir),
		}

		root.status = root.statusLogger.source
	})

	return root
}

var waitShutdown sync.WaitGroup

// Shutdown indicates that the program is exiting, and we should shutdown the progressui
//  if it's currently running
func Shutdown() {
	if root != nil {
		close(root.status)
		waitShutdown.Wait()
	}
}

func (m *Magnet) Target(name string) *Magnet {
	InitOutput()

	now := time.Now()
	vertex := &progressui.Vertex{
		Digest:  digest.FromString(name),
		Name:    name,
		Started: &now,
	}

	// the root vertex doesn't get fully added to the progress ui. So only add a parent if we're not root
	if m.parent != nil {
		vertex.Inputs = []digest.Digest{m.Vertex.Digest}
	}

	status := &progressui.SolveStatus{
		Vertexes: []*progressui.Vertex{vertex},
	}

	m.root().status <- status

	return &Magnet{
		Vertex: vertex,
		parent: m,
	}
}

var debiantFrontend = E(EnvVar{
	Key:   "DEBIAN_FRONTEND",
	Short: "Set to noninteractive or stderr to null to enable non-interactive output",
})

var CacheDir = E(EnvVar{
	Key:     "XDG_CACHE_HOME",
	Short:   "Location to store/cache build assets",
	Default: "build/cache",
})

// AbsCacheDir is the configured cache directory as an absolute path.
func AbsCacheDir() string {
	if filepath.IsAbs(CacheDir) {
		return CacheDir
	}

	wd, err := os.Getwd()
	if err != nil {
		panic(err)
	}
	return filepath.Join(wd, CacheDir)
}

func InitOutput() {
	outputOnce.Do(func() {
		if root.PrintConfig {
			root.printHeader()
		}

		var c console.Console

		if debiantFrontend != "noninteractive" {
			if cn, err := console.ConsoleFromFile(os.Stderr); err == nil {
				c = cn
			}
		}

		waitShutdown.Add(1)
		go func() {
			err := progressui.DisplaySolveStatus(
				context.TODO(),
				root.Vertex.Name,
				c,
				os.Stdout,
				root.statusLogger.destination,
			)
			if err != nil {
				panic(trace.DebugReport(err))
			}

			waitShutdown.Done()
		}()

	})
}

func (m *Magnet) root() *Magnet {
	root := m
	for root.parent != nil {
		root = root.parent
	}

	return root
}

// Complete marks the current task as complete.
func (m *Magnet) Complete(err error) {
	now := time.Now()
	m.Vertex.Completed = &now
	m.Vertex.Cached = m.cached
	m.Vertex.Error = trace.DebugReport(err)

	m.root().status <- &progressui.SolveStatus{
		Vertexes: []*progressui.Vertex{
			m.Vertex,
		},
	}
}

// SetCached marks the current task as cached when it's completed.
func (m *Magnet) SetCached(cached bool) {
	m.cached = cached
}

// Println allows writing log entries to the log output for the target.
func (m *Magnet) Println(args ...interface{}) {
	msg := fmt.Sprintln(args...)

	m.root().status <- &progressui.SolveStatus{
		Logs: []*progressui.VertexLog{
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
