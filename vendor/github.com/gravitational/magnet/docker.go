package magnet

import (
	"context"
	"fmt"
	"io/ioutil"
	"path/filepath"

	"github.com/gravitational/magnet/pkg/cp"
	"github.com/gravitational/trace"
)

type DockerConfigCommon struct {
	magnet *Magnet

	// Env are environment variables to pass to the spawned docker command
	Env map[string]string
}

type DockerConfigBuild struct {
	DockerConfigCommon
	// Always attempt to pull a newer version of the same image (Default: true)
	Pull bool
	// Compress the build context using gzip (Default: true)
	Compress bool
	// NoCache indicated to docker to avoid caching the results (Default: false)
	NoCache bool
	// Tag Name and optionally a tag in the 'name:tag' format
	Tag []string
	// BuildArgs set build-time variables
	BuildArgs map[string]string
	// Dockerfile is the path to the Dockerfile to build
	Dockerfile string
	// Target sets the target build stage to build
	Target string
	// CacheFrom is a list of images to consider as cache sources
	// https://andrewlock.net/caching-docker-layers-on-serverless-build-hosts-with-multi-stage-builds---target,-and---cache-from/
	CacheFrom []string

	// ContextFiles are paths to add to the docker context sent to the daemon
	ContextFiles []string

	// IncludePaths
	//IncludePaths []string

	// ExcludePatterns
	//ExcludePatterns []string

	// ContextCopyConfigs is a list of copy operations to build a custom docker context
	ContextCopyConfigs []cp.Config

	// TODO: Support custom build context behaviour (IE a whitelist type approach)
	// and possibly an implementation that scans and finds all go files ignoring common directories (like .git)
	// Or possibly make it easy to stage all required files into a temp directory, and pass that as a context
}

func (m *Magnet) DockerBuild() *DockerConfigBuild {
	return &DockerConfigBuild{
		DockerConfigCommon: DockerConfigCommon{
			magnet: m,
			Env: map[string]string{
				"DOCKER_BUILDKIT":   "1",
				"PROGRESS_NO_TRUNC": "1",
			},
		},
		Pull:     true,
		Compress: true,
	}
}

func (m *DockerConfigBuild) AddTag(tag string) *DockerConfigBuild {
	m.Tag = append(m.Tag, tag)
	return m
}

func (m *DockerConfigBuild) AddCacheFrom(from string) *DockerConfigBuild {
	m.CacheFrom = append(m.CacheFrom, from)
	return m
}

func (m *DockerConfigBuild) SetBuildArg(key, value string) *DockerConfigBuild {
	if m.BuildArgs == nil {
		m.BuildArgs = make(map[string]string)
	}

	m.BuildArgs[key] = value

	return m
}

func (m *DockerConfigBuild) SetEnv(key, value string) *DockerConfigBuild {
	if m.Env == nil {
		m.Env = make(map[string]string)
	}

	m.Env[key] = value

	return m
}

func (m *DockerConfigBuild) SetEnvs(envs map[string]string) *DockerConfigBuild {
	if m.Env == nil {
		m.Env = make(map[string]string)
	}

	for key, value := range envs {
		m.Env[key] = value
	}

	return m
}

func (m *DockerConfigBuild) SetDockerfile(dockerfile string) *DockerConfigBuild {
	m.Dockerfile = dockerfile
	return m
}

func (m *DockerConfigBuild) SetPull(pull bool) *DockerConfigBuild {
	m.Pull = pull
	return m
}

func (m *DockerConfigBuild) SetCompress(compress bool) *DockerConfigBuild {
	m.Compress = compress
	return m
}

func (m *DockerConfigBuild) SetNoCache(nocache bool) *DockerConfigBuild {
	m.NoCache = nocache
	return m
}

func (m *DockerConfigBuild) SetTarget(target string) *DockerConfigBuild {
	m.Target = target
	return m
}

/*
func (m *DockerConfigBuild) AddContextPath(path string) *DockerConfigBuild {
	m.ContextFiles = append(m.ContextFiles, path)
	return m
}

func (m *DockerConfigBuild) AddContextPaths(paths []string) *DockerConfigBuild {
	m.ContextFiles = append(m.ContextFiles, paths...)
	return m
}
*/

// CopyToContext creates a new docker context directory structure, including only the files that match
// the provided glob patterns.
// Notes:
// - Can be called multiple times
// - Destination will be made relative to the root directory of the context automatically
// - An unset Destination will use the same relative struct as source. Use "/" to copy to the root.
// - include/exclude patterns are optional, and all files will be copied when unset.
func (m *DockerConfigBuild) CopyToContext(src, dst string, includePatterns, excludePatterns []string) *DockerConfigBuild {
	c := cp.Config{
		Source:          src,
		Destination:     dst,
		IncludePatterns: includePatterns,
		ExcludePatterns: excludePatterns,
	}

	if len(dst) == 0 {
		c.Destination = src
	}

	m.ContextCopyConfigs = append(m.ContextCopyConfigs, c)

	return m
}

func (m *DockerConfigBuild) Build(ctx context.Context, contextPath string) error {
	args := []string{"build"}

	if m.Pull {
		args = append(args, "--pull")
	}

	if m.Compress {
		args = append(args, "--compress")
	}

	if m.NoCache {
		args = append(args, "--no-cache")
	}

	if len(m.Target) > 0 {
		args = append(args, "--target", m.Target)
	}

	for key, value := range m.BuildArgs {
		args = append(args, "--build-arg", fmt.Sprint(key, "=", value))
	}

	for _, value := range m.CacheFrom {
		args = append(args, "--cache-from", value)
	}

	for _, value := range m.Tag {
		args = append(args, "-t", value)
	}

	if len(m.ContextCopyConfigs) != 0 {
		newContextPath, err := ioutil.TempDir("", "docker-context")
		if err != nil {
			return trace.Wrap(err)
		}

		for _, c := range m.ContextCopyConfigs {
			// We want any copy operation to be relative to our context destination directory
			c.Destination = filepath.Join(newContextPath, c.Destination)

			err = cp.Copy(c)
			if err != nil {
				return trace.Wrap(err)
			}
		}

		if len(m.Dockerfile) > 0 {
			if filepath.IsAbs(m.Dockerfile) {
				err = cp.Copy(cp.Config{
					Source:      m.Dockerfile,
					Destination: filepath.Join(newContextPath, "Dockerfile"),
				})
				if err != nil {
					return trace.Wrap(err)
				}
			} else {
				err = cp.Copy(cp.Config{
					Source:      filepath.Join(contextPath, m.Dockerfile),
					Destination: filepath.Join(newContextPath, "Dockerfile"),
				})
				if err != nil {
					return trace.Wrap(err)
				}
			}

		} else {
			err = cp.Copy(cp.Config{
				Source:      filepath.Join(contextPath, "Dockerfile"),
				Destination: filepath.Join(newContextPath, "Dockerfile"),
			})
			if err != nil {
				return trace.Wrap(err)
			}
		}

		contextPath = newContextPath
	} else {
		if len(m.Dockerfile) > 0 {
			args = append(args, "-f", m.Dockerfile)
		}
	}

	// To minimize the context passed to docker, we'll copy only the needed context files
	// Right now this is very hacky, because Docker itself doesn't really expose a way to do this
	// so we copy the files we're interested into a temp dir, that is then used by docker
	// TODO: Is there a less hacky way to do this.
	/*if len(m.IncludePaths) != 0 || len(m.ExcludePatterns) != 0 {
		archiveOptions := &archive.TarOptions{
			Compression:     archive.Gzip,
			ExcludePatterns: append(m.ExcludePatterns, "build/tmp/*"),
			IncludeFiles:    m.IncludePaths,
		}

		tarArchive, err := archive.TarWithOptions(contextPath, archiveOptions)
		if err != nil {
			return trace.Wrap(err)
		}

		contextTempDir, err := ioutil.TempDir("build/tmp", "docker-context")
		if err != nil {
			return trace.Wrap(err)
		}

		contextTarPath := filepath.Join(contextTempDir, "context.tar.gz")
		contextExtractPath := filepath.Join(contextTempDir, "extract")

		err = os.MkdirAll(contextTempDir, 0755)
		if err != nil {
			return trace.Wrap(err)
		}

		archiveFH, err := os.Create(contextTarPath)
		if err != nil {
			return trace.Wrap(err)
		}

		written, err := io.Copy(archiveFH, tarArchive)
		if err != nil {
			return trace.Wrap(err)
		}

		m.magnet.Println("Wrote ", humanize.Bytes(uint64(written)), " bytes to ", contextTarPath)

		archiveFH.Close()
		tarArchive.Close()

		err = archiver.NewTarGz().Unarchive(contextTarPath, contextExtractPath)
		if err != nil {
			return trace.Wrap(err)
		}

		contextPath = filepath.Join(contextTempDir, "extract")

		if len(m.Dockerfile) > 0 {
			_, err = m.magnet.Exec().Run(context.TODO(), "cp", m.Dockerfile, filepath.Join(contextExtractPath, "Dockerfile"))
			if err != nil {
				return trace.Wrap(err)
			}
		} else {
			_, err = m.magnet.Exec().Run(context.TODO(), "cp", "Dockerfile", filepath.Join(contextExtractPath, "Dockerfile"))
			if err != nil {
				return trace.Wrap(err)
			}
		}
	} else {
		if len(m.Dockerfile) > 0 {
			args = append(args, "-f", m.Dockerfile)
		}
	}
	*/

	// TODO this is pretty nasty to create a whitelist approach
	// Use an archiver to tar up the whitelist of files, and re-extract to a temp directory, to
	// be re-tarred by Docker to pass to the docker daemon. But works as a starting point to
	// keep the docker context as minimal as possible.
	/*
		if len(m.ContextFiles) > 0 {
			if len(m.Dockerfile) > 0 {
				m.ContextFiles = append(m.ContextFiles, m.Dockerfile)
			} else {
				m.ContextFiles = append(m.ContextFiles, "Dockerfile")
			}

			contextDir, err := ioutil.TempDir("", "docker-context")
			if err != nil {
				return trace.Wrap(err)
			}

			//defer os.RemoveAll(contextDir)

			tar := archiver.NewTarGz()

			err = tar.Archive(m.ContextFiles, filepath.Join(contextDir, "context.tar.gz"))
			if err != nil {
				return trace.Wrap(err)
			}

			err = tar.Close()
			if err != nil {
				return trace.Wrap(err)
			}

			tar = archiver.NewTarGz()
			tar.Unarchive(filepath.Join(contextDir, "context.tar.gz"), filepath.Join(contextDir, "context"))

			contextPath = filepath.Join(contextDir, "context")
		}*/

	args = append(args, contextPath)

	_, err := m.magnet.Exec().SetEnvs(m.Env).Run(ctx, "docker", args...)

	return trace.Wrap(err)
}

type DockerConfigRun struct {
	DockerConfigCommon

	// Eun container in background
	Detach bool

	// User ID of spawned process
	UID string
	// Group ID of spawned process
	GID string

	// Privileged Give extended privileges to the container
	Privileged bool
	// ReadOnly mounts the containers root filesystem as read only
	ReadOnly bool
	// Automatically remove the container when it exits
	Remove bool
	// Volumes is a list of volumes to bind mount
	Volumes []string
	// Workdir sets the working directory inside the container
	WorkDir string
}

func (m *Magnet) DockerRun() *DockerConfigRun {
	return &DockerConfigRun{
		DockerConfigCommon: DockerConfigCommon{
			magnet: m,
		},
	}
}

func (m *DockerConfigRun) SetEnv(key, value string) *DockerConfigRun {
	if m.Env == nil {
		m.Env = make(map[string]string)
	}

	m.Env[key] = value

	return m
}

func (m *DockerConfigRun) SetEnvs(envs map[string]string) *DockerConfigRun {
	if m.Env == nil {
		m.Env = make(map[string]string)
	}

	for key, value := range envs {
		m.Env[key] = value
	}

	return m
}

func (m *DockerConfigRun) SetDetach(detach bool) *DockerConfigRun {
	m.Detach = detach
	return m
}

func (m *DockerConfigRun) SetUID(uid string) *DockerConfigRun {
	m.UID = uid
	return m
}

func (m *DockerConfigRun) SetGID(gid string) *DockerConfigRun {
	m.GID = gid
	return m
}

func (m *DockerConfigRun) SetPrivileged(privileged bool) *DockerConfigRun {
	m.Privileged = privileged
	return m
}

func (m *DockerConfigRun) SetReadonly(readonly bool) *DockerConfigRun {
	m.ReadOnly = readonly
	return m
}

func (m *DockerConfigRun) SetRemove(remove bool) *DockerConfigRun {
	m.Remove = remove
	return m
}

func (m *DockerConfigRun) AddVolume(volume string) *DockerConfigRun {
	m.Volumes = append(m.Volumes, volume)
	return m
}

func (m *DockerConfigRun) SetWorkDir(workdir string) *DockerConfigRun {
	m.WorkDir = workdir
	return m
}

func (m *DockerConfigRun) Run(ctx context.Context, image, cmd string, cargs ...string) error {
	args := []string{"run"}

	if m.Detach {
		args = append(args, "-d")
	}

	if len(m.UID) > 0 {
		if len(m.GID) > 0 {
			args = append(args, "-u", fmt.Sprint(m.UID, ":", m.GID))
		} else {
			args = append(args, "-u", m.UID)
		}
	}

	if m.Privileged {
		args = append(args, "--privileged")
	}

	if m.ReadOnly {
		args = append(args, "--read-only")
	}

	if m.Remove {
		args = append(args, "--rm=true")
	}

	if len(m.WorkDir) > 0 {
		args = append(args, "-w", m.WorkDir)
	}

	for _, value := range m.Volumes {
		args = append(args, "-v", value)
	}

	for key, value := range m.Env {
		args = append(args, fmt.Sprintf("--env=%v=%v", key, value))
	}

	args = append(args, image)
	args = append(args, cmd)
	args = append(args, cargs...)

	_, err := m.magnet.Exec().Run(ctx, "docker", args...)

	return trace.Wrap(err)
}
