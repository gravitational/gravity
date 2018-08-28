package archive

import (
	"archive/tar"
	"bytes"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"

	"github.com/gravitational/gravity/lib/defaults"

	"github.com/docker/docker/pkg/archive"
	dockerarchive "github.com/docker/docker/pkg/archive"
	. "gopkg.in/check.v1"
)

func TestArchive(t *testing.T) { TestingT(t) }

type S struct{}

var _ = Suite(&S{})

func (_ *S) TestTarGlob(c *C) {
	var matches []string
	var emptyResource string
	files := []*Item{
		DirItem("resources"),
		ItemFromString("resources/app.yaml", manifestBytes),
		ItemFromString("resources/resources.yaml", emptyResource),
	}
	input := MustCreateMemArchive(files)

	decompressed, err := archive.DecompressStream(input)
	c.Assert(err, IsNil)
	r := tar.NewReader(decompressed)

	err = TarGlob(r, "resources", []string{"*yaml"}, func(match string, file io.Reader) error {
		matches = append(matches, match)
		return nil
	})
	c.Assert(err, IsNil)
	c.Assert(matches, DeepEquals, []string{"app.yaml", "resources.yaml"})
}

func (_ *S) TestTarGlobCanBeAborted(c *C) {
	var matches []string
	var emptyResource string
	files := []*Item{
		DirItem("resources"),
		ItemFromString("resources/a.yaml", emptyResource),
		ItemFromString("resources/b.yaml", emptyResource),
		ItemFromString("resources/c.yaml", emptyResource),
	}
	input := MustCreateMemArchive(files)

	decompressed, err := archive.DecompressStream(input)
	c.Assert(err, IsNil)
	r := tar.NewReader(decompressed)

	err = TarGlob(r, "resources", []string{"a.yaml"}, func(match string, file io.Reader) error {
		matches = append(matches, match)
		return Abort
	})
	c.Assert(err, IsNil)
	c.Assert(matches, DeepEquals, []string{"a.yaml"})
}

func (_ *S) TestWritesTarFromItems(c *C) {
	buf := &bytes.Buffer{}
	data := "foo"
	item := ItemFromString("foo", data)
	expected := []file{
		{
			name: "foo",
			data: []byte(data),
		},
	}

	archive := NewTarAppender(buf)
	err := archive.Add(item)
	c.Assert(err, IsNil)
	archive.Close()

	AssertArchiveHasItems(c, ioutil.NopCloser(buf), nil, expected[0])
}

func (_ *S) TestCompressesDirectory(c *C) {
	var testCases = []file{
		{name: "dir", isDir: true},
		{name: "dir/file1", data: []byte("brown")},
		{name: "dir/file2", data: []byte("fox")},
	}

	var buf bytes.Buffer

	dir := c.MkDir()
	write(c, dir, testCases)
	err := CompressDirectory(dir, &buf)
	c.Assert(err, IsNil)

	AssertArchiveHasItems(c, ioutil.NopCloser(&buf), nil, testCases[0], testCases[1], testCases[2])
}

func (_ *S) TestExtractsWithoutPermissions(c *C) {
	var data = []byte("root")
	rc := ioutil.NopCloser(bytes.NewReader(data))
	items := []*Item{
		{
			Header: tar.Header{
				Mode:     defaults.SharedReadMask,
				Size:     int64(len(data)),
				Name:     "dir/file",
				Uid:      0,
				Gid:      0,
				Typeflag: tar.TypeRegA,
			},
			Data: rc,
		},
	}

	dir := c.MkDir()
	archive, err := CreateMemArchive(items)
	archive2 := bytes.NewBuffer(archive.Bytes())
	c.Assert(err, IsNil)
	c.Assert(archive, Not(IsNil))

	if os.Getuid() != 0 {
		// Only assert when not running as root
		c.Assert(dockerarchive.Untar(archive, dir, DefaultOptions()), ErrorMatches, ".*operation not permitted.*")
	}
	c.Assert(Extract(archive2, dir), IsNil)

	AssertDirHasFiles(c, dir, file{name: "dir", isDir: true}, file{name: "dir/file", data: data})
}

func write(c *C, dir string, files []file) {
	for _, file := range files {
		if file.isDir {
			c.Assert(os.MkdirAll(filepath.Join(dir, file.name), defaults.SharedDirMask), IsNil)
			continue
		}
		f, err := os.Create(filepath.Join(dir, file.name))
		c.Assert(err, IsNil)
		defer f.Close()
		_, err = f.Write(file.data)
		c.Assert(err, IsNil)
	}
}

const manifestBytes = `
apiVersion: v1
kind: Application
metadata:
  name: sample
  resourceVersion: "0.0.1"
installer:
  flavors:
    title: Test flavors
    items:
      - name: flavor1
        threshold:
          value: 1
          label: "1"
        profiles:
          master: 1
  servers:
     master:
       min_count: 1
       description: "control plane server"
       labels:
         role: master
       cpu:
         min_count: 1
       ram:
         min_total_mb: 700
       directories:
         - name: /var/lib/gravity
           min_total_mb: 1000
           fs_types: ['btrfs']`

func (r file) SameName(name string) bool {
	return r.name == name
}

func (r file) AssertFile(c *C, path string, fi os.FileInfo) {
	switch {
	case fi.IsDir():
		c.Assert(r.isDir, Equals, true)
	default:
		data, err := ioutil.ReadFile(path)
		c.Assert(err, IsNil)
		c.Assert(r.data, DeepEquals, data)
	}
}

func (r file) AssertItem(c *C, tarball *tar.Reader, hdr *tar.Header) {
	fi := hdr.FileInfo()
	switch {
	case r.isDir:
		c.Assert(fi.IsDir(), Equals, true)
	default:
		data, err := ioutil.ReadAll(tarball)
		c.Assert(err, IsNil)
		c.Assert(fi.IsDir(), Equals, false)
		c.Assert(r.data, DeepEquals, data)
	}
}

type file struct {
	name  string
	data  []byte
	isDir bool
}
