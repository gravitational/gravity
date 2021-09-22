package magnet

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"

	"github.com/gravitational/magnet/pkg/progressui"
	"github.com/gravitational/trace"
	"github.com/opencontainers/go-digest"
)

// SolveStatusLogger intercepts SolveStatus messages sent to the progressui, and is able to log the contents
// to disk for later analysis
type SolveStatusLogger struct {
	source      chan *progressui.SolveStatus
	destination chan *progressui.SolveStatus
	logger      chan *progressui.SolveStatus

	baseDir string
	time    time.Time

	writers     map[digest.Digest]io.WriteCloser
	vertexCache map[digest.Digest]progressui.Vertex

	// We may create children, but when logging we want to alias them to some parent logger
	aliases map[digest.Digest]digest.Digest
}

// newSolveStatusLogger creates a routine that copies and logs status messages to log files on disk.
func newSolveStatusLogger(baseDir string) (*SolveStatusLogger, error) {
	const statusChanSize = 128

	s := &SolveStatusLogger{
		source:      make(chan *progressui.SolveStatus),
		destination: make(chan *progressui.SolveStatus, statusChanSize),
		logger:      make(chan *progressui.SolveStatus, statusChanSize),
		baseDir:     baseDir,
		time:        time.Now(),
		writers:     make(map[digest.Digest]io.WriteCloser),
		vertexCache: make(map[digest.Digest]progressui.Vertex),
		aliases:     make(map[digest.Digest]digest.Digest),
	}

	err := os.MkdirAll(s.dirReal(), 0755)
	if err != nil {
		return nil, trace.Wrap(trace.ConvertSystemError(err))
	}

	_ = os.Remove(s.dirLink())
	err = os.Symlink(s.time.Format("20060102150405"), s.dirLink())
	if err != nil {
		return nil, trace.Wrap(trace.ConvertSystemError(err))
	}

	return s, nil
}

type redactor interface {
	redact(s []byte) []byte
}

func (s *SolveStatusLogger) start(redactor redactor) {
	go s.tee(redactor)
	go s.writeLogs()
}

func (s *SolveStatusLogger) dirReal() string {
	return filepath.Join(s.baseDir, s.time.Format("20060102150405"))
}

func (s *SolveStatusLogger) dirLink() string {
	return filepath.Join(s.baseDir, "latest")
}

func (s *SolveStatusLogger) tee(redactor redactor) {
	for {
		status, ok := <-s.source
		if !ok {
			close(s.destination)
			close(s.logger)

			return
		}

		// Do internal redacting of secrets from any log output, to try and reduce the risk of accidental
		// logging of secrets
		for i := range status.Logs {
			status.Logs[i].Data = redactor.redact(status.Logs[i].Data)
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

func (s *SolveStatusLogger) alias(d digest.Digest) digest.Digest {
	if digest, ok := s.aliases[d]; ok {
		return digest
	}
	return d
}

func (s *SolveStatusLogger) writeLogs() {
	for status := range s.logger {
		for _, vertex := range status.Vertexes {
			_, ok := s.writers[s.alias(vertex.Digest)]
			if !ok {
				err := os.MkdirAll(s.dirReal(), 0755)
				if err != nil {
					panic(trace.DebugReport(trace.ConvertSystemError(err)))
				}

				writer, err := os.OpenFile(filepath.Join(s.dirReal(), vertex.Name), os.O_WRONLY|os.O_CREATE, 0644)
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

	for _, writer := range s.writers {
		writer.Close()
	}
}

func (s *SolveStatusLogger) logVertexStatus(status *progressui.VertexStatus) {
	// for now, do nothing. Vertex Status is mainly for reporting progress status, which we likely want to rate limit
	// if we capture that somehow within the logs
}

func (s *SolveStatusLogger) logVertexLog(log *progressui.VertexLog) {
	writer, ok := s.writers[log.Vertex]
	if !ok {
		return
	}

	// TODO: Do we just want to log/write raw data, or do we want to include timestamp and possibly stream
	// (stdout/stderr) information
	_, err := writer.Write(log.Data)
	if err != nil {
		panic(trace.DebugReport(trace.ConvertSystemError(err)))
	}
}

func (s *SolveStatusLogger) logVertex(vertex *progressui.Vertex) {
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

		if vertex.Error != "" {
			_, err = writer.Write([]byte(fmt.Sprintln("Error:", vertex.Error)))
			if err != nil {
				panic(trace.DebugReport(trace.ConvertSystemError(err)))
			}
		}

		_, err = writer.Write([]byte("-----\n"))
		if err != nil {
			panic(trace.DebugReport(trace.ConvertSystemError(err)))
		}

		return
	}

	_, err := writer.Write([]byte("-----\n"))
	if err != nil {
		panic(trace.DebugReport(trace.ConvertSystemError(err)))
	}

	if cachedVertex.Name != vertex.Name {
		_, err = writer.Write([]byte(fmt.Sprintf("Vertex: Name %v -> %v\n", cachedVertex.Name, vertex.Name)))
		if err != nil {
			panic(trace.DebugReport(trace.ConvertSystemError(err)))
		}
	}

	if cachedVertex.Cached != vertex.Cached {
		_, err = writer.Write([]byte(fmt.Sprintf("Vertex: Cached %v -> %v\n", cachedVertex.Cached, vertex.Cached)))
		if err != nil {
			panic(trace.DebugReport(trace.ConvertSystemError(err)))
		}
	}

	if vertex.Completed != nil && cachedVertex.Completed != vertex.Completed {
		_, err = writer.Write([]byte(fmt.Sprintf("Vertex: Completed %v -> %v\n", cachedVertex.Completed, vertex.Completed)))
		if err != nil {
			panic(trace.DebugReport(trace.ConvertSystemError(err)))
		}

		if vertex.Started != nil {
			_, err = writer.Write([]byte(fmt.Sprintf("Vertex: Duration %v\n", vertex.Completed.Sub(*vertex.Started))))
			if err != nil {
				panic(trace.DebugReport(trace.ConvertSystemError(err)))
			}
		} else if cachedVertex.Started != nil {
			_, err = writer.Write([]byte(fmt.Sprintf("Vertex: Duration %v\n", vertex.Completed.Sub(*cachedVertex.Started))))
			if err != nil {
				panic(trace.DebugReport(trace.ConvertSystemError(err)))
			}
		}
	}

	if vertex.Started != nil && cachedVertex.Started != vertex.Started {
		_, err = writer.Write([]byte(fmt.Sprintf("Vertex: Started %v -> %v\n", cachedVertex.Started, vertex.Started)))
		if err != nil {
			panic(trace.DebugReport(trace.ConvertSystemError(err)))
		}
	}

	if cachedVertex.Error != vertex.Error {
		_, err = writer.Write([]byte(fmt.Sprintf("Vertex: Error %v -> %v\n", cachedVertex.Error, vertex.Error)))
		if err != nil {
			panic(trace.DebugReport(trace.ConvertSystemError(err)))
		}
	}

	_, err = writer.Write([]byte("-----\n"))
	if err != nil {
		panic(trace.DebugReport(trace.ConvertSystemError(err)))
	}
}
