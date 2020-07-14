package cp

import (
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"syscall"

	"github.com/gravitational/trace"
)

type Config struct {
	// IncludePatterns are golang file globs for files to include in the copy
	IncludePatterns []string
	// ExcludePatterns are golang file globs for files to exclude that matched an include pattern
	ExcludePatterns []string
	// Source is the base directory to copy from
	Source string
	// Destination is the bast directory to copy to
	Destination string
}

func Copy(c Config) error {
	err := c.checkAndSetDefaults()
	if err != nil {
		return trace.Wrap(err)
	}

	sfi, err := os.Stat(c.Source)
	if err != nil {
		return trace.ConvertSystemError(err)
	}

	if sfi.IsDir() {
		return trace.Wrap(c.copyDir(c.Source, c.Destination))
	}

	return trace.Wrap(CopyFile(c.Source, c.Destination))
}

// copyDir
// based on https://stackoverflow.com/questions/51779243/copy-a-folder-in-go
func (c Config) copyDir(src, dst string) error {
	entries, err := ioutil.ReadDir(src)
	if err != nil {
		return err
	}

	if err := CreateIfNotExists(dst, 0755); err != nil {
		return err
	}

	for _, entry := range entries {
		sourcePath := filepath.Join(src, entry.Name())
		destPath := filepath.Join(dst, entry.Name())

		filtered, err := c.checkIsFiltered(sourcePath)
		if err != nil {
			return trace.Wrap(err)
		}

		if filtered {
			return nil
		}

		fileInfo, err := os.Stat(sourcePath)
		if err != nil {
			return trace.Wrap(err).AddField("src", sourcePath)
		}

		stat, ok := fileInfo.Sys().(*syscall.Stat_t)
		if !ok {
			return trace.BadParameter("failed to get raw syscall.Stat_t data for '%s'", sourcePath)
		}

		switch fileInfo.Mode() & os.ModeType {
		case os.ModeDir:
			if err := CreateIfNotExists(destPath, fileInfo.Mode()); err != nil {
				return trace.Wrap(err)
			}

			if err := c.copyDir(sourcePath, destPath); err != nil {
				return trace.Wrap(err)
			}
		case os.ModeSymlink:
			if err := CopySymLink(sourcePath, destPath); err != nil {
				return trace.Wrap(err)
			}
		default:
			if err := CopyFile(sourcePath, destPath); err != nil {
				return trace.Wrap(err)
			}
		}

		// Only root is able to chown to a different user
		if os.Getuid() == 0 {
			if err := os.Lchown(destPath, int(stat.Uid), int(stat.Gid)); err != nil {
				return trace.Wrap(err).AddFields(map[string]interface{}{
					"src": sourcePath,
					"uid": stat.Uid,
					"gid": stat.Gid,
				})
			}
		}

		isSymlink := entry.Mode()&os.ModeSymlink != 0
		if !isSymlink {
			if err := os.Chmod(destPath, entry.Mode()); err != nil {
				return trace.Wrap(trace.ConvertSystemError(err)).AddField("dst", destPath)
			}
		}
	}

	return nil
}

// CopyFile copies a file from source to destination, using hardlinks if it can
// ignores Include/Exclude patterns, as referencing a file copy directly is expected to be wanted
// based on https://stackoverflow.com/questions/21060945/simple-way-to-copy-a-file-in-golang
func CopyFile(src, dst string) error {
	sfi, err := os.Stat(src)
	if err != nil {
		return trace.Wrap(trace.ConvertSystemError(err)).AddField("src", src)
	}

	if !sfi.Mode().IsRegular() {
		// cannot copy non-regular files (e.g., directories,
		// symlinks, devices, etc.)
		// TODO: will eventually need to support symlinks
		return trace.BadParameter("CopyFile: non-regular source file %s (%q)", sfi.Name(), sfi.Mode().String())
	}

	dfi, err := os.Stat(dst)
	if err != nil {
		if !os.IsNotExist(err) {
			return trace.Wrap(trace.ConvertSystemError(err)).AddField("dst", dst)
		}
	} else {
		if !(dfi.Mode().IsRegular()) {
			return trace.BadParameter("CopyFile: non-regular destination file %s (%q)", dfi.Name(), dfi.Mode().String())
		}
		if os.SameFile(sfi, dfi) {
			return nil
		}
	}

	if err = os.Link(src, dst); err == nil {
		return nil
	}

	in, err := os.Open(src)
	if err != nil {
		return trace.Wrap(trace.ConvertSystemError(err)).AddFields(map[string]interface{}{
			"src": src,
			"dst": dst,
		})
	}

	defer in.Close()

	out, err := os.Create(dst)
	if err != nil {
		return trace.Wrap(trace.ConvertSystemError(err)).AddField("dst", dst)
	}

	_, err = io.Copy(out, in)
	if err != nil {
		return trace.Wrap(trace.ConvertSystemError(err)).AddField("dst", dst)
	}

	err = out.Sync()
	if err != nil {
		return trace.Wrap(trace.ConvertSystemError(err)).AddField("dst", dst)
	}

	err = out.Close()
	if err != nil {
		return trace.Wrap(trace.ConvertSystemError(err)).AddField("dst", dst)
	}

	return trace.Wrap(os.Chmod(dst, sfi.Mode()))
}

func CopySymLink(source, dest string) error {
	link, err := os.Readlink(source)
	if err != nil {
		return err
	}

	return os.Symlink(link, dest)
}

func CreateIfNotExists(dir string, perm os.FileMode) error {
	if Exists(dir) {
		return nil
	}

	if err := os.MkdirAll(dir, perm); err != nil {
		return trace.BadParameter("failed to create directory: '%s', error: '%s'", dir, err.Error())
	}

	return nil
}

func Exists(filePath string) bool {
	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		return false
	}

	return true
}

func (c Config) checkIsFiltered(src string) (bool, error) {
	for _, p := range c.ExcludePatterns {
		match, err := filepath.Match(p, src)
		if err != nil {
			return false, trace.ConvertSystemError(err)
		}

		if match {
			return true, nil
		}
	}

	for _, p := range c.IncludePatterns {
		match, err := filepath.Match(p, src)
		if err != nil {
			return false, trace.ConvertSystemError(err)
		}

		if match {
			return false, nil
		}
	}

	// If an include list wasn't set, assume we want to include everything
	if len(c.IncludePatterns) == 0 {
		return false, nil
	}

	return true, nil
}

func (c Config) checkAndSetDefaults() error {
	for _, p := range c.IncludePatterns {
		if _, err := filepath.Match(p, "."); err != nil {
			return trace.Wrap(err)
		}
	}

	for _, p := range c.ExcludePatterns {
		if _, err := filepath.Match(p, "."); err != nil {
			return trace.Wrap(err)
		}
	}

	if len(c.Source) == 0 {
		return trace.BadParameter("Expected source to be set")
	}

	if len(c.Destination) == 0 {
		return trace.BadParameter("Expected destination to be set")
	}

	_, err := os.Stat(c.Source)
	if err != nil {
		return trace.ConvertSystemError(err)
	}

	return nil
}
