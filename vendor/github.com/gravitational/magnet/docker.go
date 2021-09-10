package magnet

import (
	"context"
	"fmt"
	"io/ioutil"
	"path/filepath"

	"github.com/gravitational/magnet/pkg/cp"
	"github.com/gravitational/trace"
)

// DockerConfigCommon holds common configuration for docker commands.
type DockerConfigCommon struct {
	target *MagnetTarget

	// Env are environment variables to pass to the spawned docker command
	Env map[string]string
}

// DockerConfigBuild holds configuration for building docker containers.
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

	// ContextCopyConfigs is a list of copy operations to build a custom docker context
	ContextCopyConfigs []cp.Config
}

// DockerBuild creates a command for building a docker container using buildkit.
func (m *MagnetTarget) DockerBuild() *DockerConfigBuild {
	return &DockerConfigBuild{
		DockerConfigCommon: DockerConfigCommon{
			target: m,
			Env: map[string]string{
				"DOCKER_BUILDKIT":   "1",
				"PROGRESS_NO_TRUNC": "1",
			},
		},
		Pull:     true,
		Compress: true,
	}
}

// AddTag adds a name and optionally a tag in the 'name:tag' format. Can be added multiple times.
func (m *DockerConfigBuild) AddTag(tag string) *DockerConfigBuild {
	m.Tag = append(m.Tag, tag)
	return m
}

// AddCacheFrom adds an image to consider as a cache source.
func (m *DockerConfigBuild) AddCacheFrom(from string) *DockerConfigBuild {
	m.CacheFrom = append(m.CacheFrom, from)
	return m
}

// SetBuildArg sets a build argument to pass to the build provess.
func (m *DockerConfigBuild) SetBuildArg(key, value string) *DockerConfigBuild {
	if m.BuildArgs == nil {
		m.BuildArgs = make(map[string]string)
	}

	m.BuildArgs[key] = value

	return m
}

// SetEnv sets an environment variable on the docker build command.
func (m *DockerConfigBuild) SetEnv(key, value string) *DockerConfigBuild {
	if m.Env == nil {
		m.Env = make(map[string]string)
	}

	m.Env[key] = value

	return m
}

// SetEnvs sets environmanet variables on the docker build command.
func (m *DockerConfigBuild) SetEnvs(envs map[string]string) *DockerConfigBuild {
	if m.Env == nil {
		m.Env = make(map[string]string)
	}

	for key, value := range envs {
		m.Env[key] = value
	}

	return m
}

// SetDockerfile sets the name of the Dockerfile (Default is PATH/Dockerfile).
func (m *DockerConfigBuild) SetDockerfile(dockerfile string) *DockerConfigBuild {
	m.Dockerfile = dockerfile
	return m
}

// SetPull attempts to always pull a newer version of base images.
func (m *DockerConfigBuild) SetPull(pull bool) *DockerConfigBuild {
	m.Pull = pull
	return m
}

// SetCompress compresses the build context when passing to the docker daemon.
func (m *DockerConfigBuild) SetCompress(compress bool) *DockerConfigBuild {
	m.Compress = compress
	return m
}

// SetNoCache does not use cache when building images.
func (m *DockerConfigBuild) SetNoCache(nocache bool) *DockerConfigBuild {
	m.NoCache = nocache
	return m
}

// SetTarget sets the target build stage to build.
func (m *DockerConfigBuild) SetTarget(target string) *DockerConfigBuild {
	m.Target = target
	return m
}

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

// Build calls docker to build a container image.
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

	args = append(args, contextPath)

	_, err := m.target.Exec().SetEnvs(m.Env).Run(ctx, "docker", args...)

	return trace.Wrap(err)
}

// DockerBindMount represents a mount point that can be passed when running a docker container
type DockerBindMount struct {
	// Type is the docker type [mount(default), volume, tmpfs]
	// https://docs.docker.com/storage/bind-mounts/
	Type string
	// Source (Bind mount only) is the path to the file or directory on the Docker daemon host.
	Source string
	// Destination is the path where the file or directory is mounted within the container
	Destination string
	// Readonly causes the mount point to be mounted readonly
	Readonly bool
	// BindPropagation changes the bind propagation [rprivate, private, rshared, shared, rslave, slave]
	// https://docs.docker.com/storage/bind-mounts/#configure-bind-propagation
	BindPropagation string
	// Consistency applies to Mac only and is ignored on other platforms. [consistent, delegated, cached]
	Consistency string
}

func (b DockerBindMount) arg() string {
	if b.Type == "" {
		b.Type = "bind"
	}

	arg := "type=" + b.Type + ",source=" + b.Source + ",target=" + b.Destination
	if b.Readonly {
		arg += ",readonly"
	}

	if b.BindPropagation != "" {
		arg += ",bind-propagation=" + b.BindPropagation
	}

	if b.Consistency != "" {
		arg += ",consistency=" + b.Consistency
	}

	return arg
}

// DockerConfigRun holds configuration used to run a docker container.
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
	Volumes []DockerBindMount
	// Workdir sets the working directory inside the container
	WorkDir string
	// Network for the container to connect to
	Network string
}

// DockerRun creates a command builder for running a docker container.
func (m *MagnetTarget) DockerRun() *DockerConfigRun {
	return &DockerConfigRun{
		DockerConfigCommon: DockerConfigCommon{
			target: m,
		},
	}
}

// SetEnv passed an environment variable to the running container.
func (m *DockerConfigRun) SetEnv(key, value string) *DockerConfigRun {
	if m.Env == nil {
		m.Env = make(map[string]string)
	}

	m.Env[key] = value

	return m
}

// SetEnvs passes environment variables to the running container.
func (m *DockerConfigRun) SetEnvs(envs map[string]string) *DockerConfigRun {
	if m.Env == nil {
		m.Env = make(map[string]string)
	}

	for key, value := range envs {
		m.Env[key] = value
	}

	return m
}

// SetDetach runs the container in the background.
func (m *DockerConfigRun) SetDetach(detach bool) *DockerConfigRun {
	m.Detach = detach
	return m
}

// SetUID sets the user id of the container.
func (m *DockerConfigRun) SetUID(uid string) *DockerConfigRun {
	m.UID = uid
	return m
}

// SetGID sets the group id of the container.
func (m *DockerConfigRun) SetGID(gid string) *DockerConfigRun {
	m.GID = gid
	return m
}

// SetPrivileged gives extended privileges to the container.
func (m *DockerConfigRun) SetPrivileged(privileged bool) *DockerConfigRun {
	m.Privileged = privileged
	return m
}

// SetReadonly sets the containers rootfs to readonly.
func (m *DockerConfigRun) SetReadonly(readonly bool) *DockerConfigRun {
	m.ReadOnly = readonly
	return m
}

// SetRemove automatically removes the container when the container exits.
func (m *DockerConfigRun) SetRemove(remove bool) *DockerConfigRun {
	m.Remove = remove
	return m
}

// AddVolume attaches a set of filesystem mounts to the container.
func (m *DockerConfigRun) AddVolume(volumes ...DockerBindMount) *DockerConfigRun {
	m.Volumes = append(m.Volumes, volumes...)
	return m
}

// SetWorkDir sets the working directory inside the container.
func (m *DockerConfigRun) SetWorkDir(workdir string) *DockerConfigRun {
	m.WorkDir = workdir
	return m
}

// SetNetwork overrides the network container will connect to
func (m *DockerConfigRun) SetNetwork(network string) *DockerConfigRun {
	m.Network = network
	return m
}

// Run calls docker by cli to run the configured container.
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

	if len(m.Network) > 0 {
		args = append(args, "--network", m.Network)
	}

	for _, value := range m.Volumes {
		args = append(args, "--mount", value.arg())
	}

	for key, value := range m.Env {
		args = append(args, fmt.Sprintf("--env=%v=%v", key, value))
	}

	args = append(args, image)
	args = append(args, cmd)
	args = append(args, cargs...)

	_, err := m.target.Exec().Run(ctx, "docker", args...)

	return trace.Wrap(err)
}
