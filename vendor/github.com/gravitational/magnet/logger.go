package magnet

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/gravitational/trace"
	"github.com/moby/buildkit/client"
	"github.com/opencontainers/go-digest"
)

type SolveStatusLogger struct {
	source      chan *client.SolveStatus
	destination chan *client.SolveStatus
	logger      chan *client.SolveStatus

	baseDir     string
	writers     map[digest.Digest]io.WriteCloser
	vertexCache map[digest.Digest]client.Vertex
}

// newSolveStatusLogger creates a routine that copies and logs status messages to log files on disk.
func newSolveStatusLogger(baseDir string) *SolveStatusLogger {
	const statusChanSize = 128

	s := &SolveStatusLogger{
		source:      make(chan *client.SolveStatus),
		destination: make(chan *client.SolveStatus, statusChanSize),
		logger:      make(chan *client.SolveStatus, statusChanSize),
		baseDir:     baseDir,
		writers:     make(map[digest.Digest]io.WriteCloser),
		vertexCache: make(map[digest.Digest]client.Vertex),
	}

	go s.tee()
	go s.writeLogs()

	return s
}

func (s *SolveStatusLogger) tee() {
	for {
		status, ok := <-s.source
		if !ok {
			close(s.destination)
			close(s.logger)

			for _, writer := range s.writers {
				writer.Close()
			}

			return
		}

		// Do internal redacting of secrets from any log output, to try and reduce the risk of accidental
		// logging of secrets
		for i := range status.Logs {
			for _, value := range EnvVars {
				if value.Secret && len(value.Value) > 0 {
					status.Logs[i].Data = []byte(strings.ReplaceAll(string(status.Logs[i].Data), value.Value, "<redacted>"))
				}
			}
		}

		select {
		case s.destination <- status:
		default:
		}
		select {
		case s.logger <- status:
		default:
		}
	}
}

func (s *SolveStatusLogger) writeLogs() {
	for status := range s.logger {
		for _, vertex := range status.Vertexes {
			_, ok := s.writers[vertex.Digest]
			if !ok {
				err := os.MkdirAll(s.baseDir, 0755)
				if err != nil {
					panic(trace.DebugReport(trace.ConvertSystemError(err)))
				}

				writer, err := os.OpenFile(filepath.Join(s.baseDir, vertex.Name), os.O_WRONLY|os.O_CREATE, 0644)
				if err != nil {
					panic(trace.DebugReport(trace.ConvertSystemError(err)))
				}

				s.writers[vertex.Digest] = writer
			}

			s.logVertex(vertex)
		}

		for _, status := range status.Statuses {
			s.logVertexStatus(status)
		}

		for _, log := range status.Logs {
			s.logVertexLog(log)
		}
	}
}

func (s *SolveStatusLogger) logVertexStatus(status *client.VertexStatus) {
	// for now, do nothing. Vertex Status is mainly for reporting progress status, which we likely want to rate limit
}

func (s *SolveStatusLogger) logVertexLog(log *client.VertexLog) {
	writer, ok := s.writers[log.Vertex]
	if !ok {
		return
	}

	// TODO: Do we just want to log/write raw data, or do we want to include timestamp and possibly stream information
	_, err := writer.Write(log.Data)
	if err != nil {
		panic(trace.DebugReport(trace.ConvertSystemError(err)))
	}
}

func (s *SolveStatusLogger) logVertex(vertex *client.Vertex) {
	writer, ok := s.writers[vertex.Digest]
	if !ok {
		return
	}

	cachedVertex, ok := s.vertexCache[vertex.Digest]
	if !ok {
		// this is the first time we've seen a vertex, so write it and cache it
		s.vertexCache[vertex.Digest] = *vertex

		_, err := writer.Write([]byte(fmt.Sprintln("Name:", vertex.Name)))
		if err != nil {
			panic(trace.DebugReport(trace.ConvertSystemError(err)))
		}

		_, err = writer.Write([]byte(fmt.Sprintln("Digest:", vertex.Digest)))
		if err != nil {
			panic(trace.DebugReport(trace.ConvertSystemError(err)))
		}

		_, err = writer.Write([]byte(fmt.Sprintln("Cached:", vertex.Cached)))
		if err != nil {
			panic(trace.DebugReport(trace.ConvertSystemError(err)))
		}

		_, err = writer.Write([]byte(fmt.Sprintln("Started:", vertex.Started)))
		if err != nil {
			panic(trace.DebugReport(trace.ConvertSystemError(err)))
		}

		_, err = writer.Write([]byte(fmt.Sprintln("Completed:", vertex.Completed)))
		if err != nil {
			panic(trace.DebugReport(trace.ConvertSystemError(err)))
		}

		_, err = writer.Write([]byte(fmt.Sprintln("Error:", vertex.Error)))
		if err != nil {
			panic(trace.DebugReport(trace.ConvertSystemError(err)))
		}

		return
	}

	if cachedVertex.Name != vertex.Name {
		_, err := writer.Write([]byte(fmt.Sprintf("Vertex: Name %v -> %v\n", cachedVertex.Name, vertex.Name)))
		if err != nil {
			panic(trace.DebugReport(trace.ConvertSystemError(err)))
		}
	}

	if cachedVertex.Cached != vertex.Cached {
		_, err := writer.Write([]byte(fmt.Sprintf("Vertex: Cached %v -> %v\n", cachedVertex.Cached, vertex.Cached)))
		if err != nil {
			panic(trace.DebugReport(trace.ConvertSystemError(err)))
		}
	}

	if vertex.Completed != nil && cachedVertex.Completed != vertex.Completed {
		_, err := writer.Write([]byte(fmt.Sprintf("Vertex: Completed %v -> %v\n", cachedVertex.Completed, vertex.Completed)))
		if err != nil {
			panic(trace.DebugReport(trace.ConvertSystemError(err)))
		}

		if vertex.Started != nil {
			_, err := writer.Write([]byte(fmt.Sprintf("Vertex: Duration %v\n", vertex.Completed.Sub(*vertex.Started))))
			if err != nil {
				panic(trace.DebugReport(trace.ConvertSystemError(err)))
			}
		} else if cachedVertex.Started != nil {
			_, err := writer.Write([]byte(fmt.Sprintf("Vertex: Duration %v\n", vertex.Completed.Sub(*cachedVertex.Started))))
			if err != nil {
				panic(trace.DebugReport(trace.ConvertSystemError(err)))
			}
		}
	}

	if vertex.Started != nil && cachedVertex.Started != vertex.Started {
		_, err := writer.Write([]byte(fmt.Sprintf("Vertex: Started %v -> %v\n", cachedVertex.Started, vertex.Started)))
		if err != nil {
			panic(trace.DebugReport(trace.ConvertSystemError(err)))
		}
	}

	if cachedVertex.Error != vertex.Error {
		_, err := writer.Write([]byte(fmt.Sprintf("Vertex: Error %v -> %v\n", cachedVertex.Error, vertex.Error)))
		if err != nil {
			panic(trace.DebugReport(trace.ConvertSystemError(err)))
		}
	}
}
