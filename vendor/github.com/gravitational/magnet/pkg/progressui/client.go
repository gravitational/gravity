package progressui

import (
	"time"

	digest "github.com/opencontainers/go-digest"
)

// Duplicate buildkit structures internally, so that we're basically compatible with buildkit, but don't need to
// vendor all the buildkit / docker libraries to function

type Vertex struct {
	Digest    digest.Digest
	Inputs    []digest.Digest
	Name      string
	Started   *time.Time
	Completed *time.Time
	Cached    bool
	Error     string
}

type VertexStatus struct {
	ID        string
	Vertex    digest.Digest
	Name      string
	Total     int64
	Current   int64
	Timestamp time.Time
	Started   *time.Time
	Completed *time.Time
}

type VertexLog struct {
	Vertex    digest.Digest
	Stream    int
	Data      []byte
	Timestamp time.Time
}

type SolveStatus struct {
	Vertexes []*Vertex
	Statuses []*VertexStatus
	Logs     []*VertexLog
}
