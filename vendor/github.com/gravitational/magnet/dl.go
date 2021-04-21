package magnet

import (
	"crypto/sha256"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"time"

	"github.com/gravitational/magnet/pkg/progressui"
	"github.com/gravitational/trace"
	"github.com/opencontainers/go-digest"
	yaml "gopkg.in/yaml.v2"
)

type downloadMetadata struct {
	ETag    string
	SHA2Sum string
}

// Download begins a download of a url but doesn't block.
// Returns a future that when called will block until it can return the path to the file on disk or an error.
func (m *MagnetTarget) DownloadFuture(url string) func() (url string, path string, err error) {
	errC := make(chan error, 1)

	var path string

	go func() {
		p, err := m.Download(url)
		path = p
		errC <- err
	}()

	return func() (string, string, error) {
		err := <-errC
		if err != nil {
			return url, "", trace.Wrap(err)
		}
		return url, path, nil
	}
}

// Download will download a file from a remote URL. It's optimized for working with a local cache, and will send
// request headers to the upstream server and only download the file if cached or missing from the local cache.
func (m *MagnetTarget) Download(url string) (path string, err error) {
	progress := dlProgressWriter{
		m:   m,
		url: url,
	}
	progress.Init()

	path = filepath.Join(m.root.cacheDir(), "dl", digest.FromString(url).String())

	metadata, err := getMetadata(path)
	if err != nil && !trace.IsNotFound(err) {
		return "", trace.Wrap(err)
	}

	// validate the checksum on disk matches our recorded checksum.
	// If it doesn't match, treat this as if the file doesn't exist
	if !validateChecksum(path, metadata.SHA2Sum) {
		metadata = downloadMetadata{}
	}

	resp, err := httpGetRequest(url, metadata.ETag)
	if err != nil {
		return "", trace.Wrap(err)
	}

	defer func() {
		_ = resp.Body.Close()
	}()

	if resp.StatusCode == http.StatusNotModified {
		progress.Complete()
		return path, nil
	}

	if resp.StatusCode != http.StatusOK {
		return "", trace.BadParameter("Unexpected status code: %v", resp.StatusCode).AddField("url", url)
	}

	err = os.MkdirAll(filepath.Dir(path), 0755)
	if err != nil {
		return "", trace.Wrap(err).AddField("path", filepath.Dir(path))
	}

	out, err := os.Create(path)
	if err != nil {
		return "", trace.Wrap(err).AddField("path", path)
	}

	defer func() {
		_ = out.Close()
	}()

	// ignore errors, we don't care if we don't have the content-length
	progress.total, _ = strconv.ParseInt(resp.Header.Get("Content-Length"), 10, 64)
	progress.Init() // Redo Init once we know how much to download

	m.Println("Total download: ", progress.total)

	// tee the file
	// - calculate the checksum as the file is download for our metadata
	// - report the download progress to our progress writer
	reader := io.TeeReader(resp.Body, out)
	reader = io.TeeReader(reader, &progress)

	hash := sha256.New()

	_, err = io.Copy(hash, reader)
	if err != nil {
		return "", trace.Wrap(err).AddField("url", url)
	}

	err = writeMetadata(downloadMetadata{
		ETag:    resp.Header.Get("ETag"),
		SHA2Sum: fmt.Sprintf("%x", hash.Sum(nil)),
	}, path)
	if err != nil {
		return "", trace.Wrap(err).AddField("path", path)
	}

	progress.Complete()
	return path, nil
}

type dlProgressWriter struct {
	m       *MagnetTarget
	url     string
	total   int64
	current int64
}

func (d *dlProgressWriter) Init() {
	vertexStatus := &progressui.VertexStatus{
		ID:        d.url,
		Vertex:    d.m.vertex.Digest,
		Total:     d.total,
		Current:   d.current,
		Timestamp: time.Now(),
		Started:   d.m.vertex.Started,
	}
	status := &progressui.SolveStatus{
		Statuses: []*progressui.VertexStatus{vertexStatus},
	}

	d.m.root.status <- status
}

func (d *dlProgressWriter) Complete() {
	now := time.Now()
	vertexStatus := &progressui.VertexStatus{
		ID:        d.url,
		Vertex:    d.m.vertex.Digest,
		Total:     d.total,
		Current:   d.current,
		Timestamp: time.Now(),
		Started:   d.m.vertex.Started,
		Completed: &now,
	}
	status := &progressui.SolveStatus{
		Statuses: []*progressui.VertexStatus{vertexStatus},
	}

	d.m.root.status <- status
}

func (d *dlProgressWriter) Write(data []byte) (int, error) {
	d.current += int64(len(data))

	vertexStatus := &progressui.VertexStatus{
		ID:        d.url,
		Vertex:    d.m.vertex.Digest,
		Total:     d.total,
		Current:   d.current,
		Timestamp: time.Now(),
		Started:   d.m.vertex.Started,
	}
	status := &progressui.SolveStatus{
		Statuses: []*progressui.VertexStatus{vertexStatus},
	}

	d.m.root.status <- status

	return len(data), nil
}

func httpGetRequest(url, etag string) (*http.Response, error) {
	client := &http.Client{}

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if etag != "" {
		req.Header.Add("If-None-Match", etag)
	}

	resp, err := client.Do(req)

	return resp, trace.Wrap(err)
}

func getMetadata(path string) (downloadMetadata, error) {
	var result downloadMetadata

	dat, err := ioutil.ReadFile(metadataPath(path))
	if err != nil {
		return result, trace.Wrap(err)
	}

	err = yaml.Unmarshal(dat, &result)

	return result, trace.Wrap(err)
}

func metadataPath(path string) string {
	return fmt.Sprintf("%v.wcache", path)
}

func writeMetadata(metadata downloadMetadata, path string) error {
	buf, err := yaml.Marshal(metadata)
	if err != nil {
		return trace.Wrap(err)
	}

	return trace.Wrap(ioutil.WriteFile(metadataPath(path), buf, 0600))
}

func validateChecksum(path, checksum string) bool {
	f, err := os.Open(path)
	if err != nil {
		return false
	}

	hash := sha256.New()

	_, err = io.Copy(hash, f)
	if err != nil {
		return false
	}

	return fmt.Sprintf("%x", hash.Sum(nil)) == checksum
}
