package magnet

import (
	"context"
	"fmt"
	"go/build"
	"os"
	"path/filepath"
	"strings"

	"github.com/gravitational/trace"
)

type GolangConfigCommon struct {
	// BuildContainer is the container image to use when running go commands
	BuildContainer string

	// GOOS to pass to the go compiler as an env variable
	GOOS string
	// GOARCH to pass to the go compiler as an env variable
	GOARCH string

	// Env is a set of environment variables to pass to the compiler
	Env map[string]string

	// Rebuilds forces rebuilding of packages that are already up to date
	Rebuild bool

	// Race enables data race detection
	Race bool

	// ParallelTasks is the number of programs, such as build commands or test binaries
	// that can be run in parallel. Defaults to number of CPUs available.
	ParallelTasks *int

	// DryRun print the commands but do not run them
	DryRun bool

	// Verbose prints the name of packages as they are compiled
	Verbose bool

	// ASMFlags is a list of arguments to pass on each go tool asm invocation.
	// ASMFlags []string

	// BuildMode is the go build mode to use (see go help buildmode)
	BuildMode string

	// Compiler is the name of the compiler to use (gccgo or gc)
	// Compiler string

	//GCCGOFlags is a list of arguments to pass on each gccfo compiler/linker invocation
	GCCGOFlags []string

	// GCFlags is a list of arguments to pass on each go tool compile invocation
	GCFlags []string

	// InstallSuffix is a suffix to use in the name of the package installation directory
	// InstallSuffix string

	// LDFlags is a list of arguments to pass on each go tool link invocation
	LDFlags []string

	// ModMode is the module download mode (readonly or vendor).
	// Use `go help modules` for more information.
	ModMode string

	// PkgDir is used to install and load packages from dir instead of the usual locations
	//PkgDir string

	// Tags is a list of build tags to consider as satisified during the build
	Tags []string
}

func (m *GolangConfigCommon) genFlags() []string {
	cmd := []string{}

	if len(m.LDFlags) > 0 {
		cmd = append(cmd, "-ldflags", strings.Join(m.LDFlags, " "))
	}

	if m.ParallelTasks != nil {
		cmd = append(cmd, "-n", fmt.Sprint(m.ParallelTasks))
	}

	if m.Rebuild {
		cmd = append(cmd, "-a")
	}

	if m.DryRun {
		cmd = append(cmd, "-n")
	}

	if m.Race {
		cmd = append(cmd, "-race")
	}

	if m.Verbose {
		cmd = append(cmd, "-v")
	}

	if len(m.BuildMode) != 0 {
		cmd = append(cmd, "-buildmode", m.BuildMode)
	}

	if len(m.GCFlags) != 0 {
		cmd = append(cmd, "-gcflags", strings.Join(m.GCFlags, " "))
	}

	if len(m.LDFlags) != 0 {
		cmd = append(cmd, "-gcflags", strings.Join(m.GCFlags, " "))
	}

	if len(m.ModMode) != 0 {
		cmd = append(cmd, "-mod", m.ModMode)
	}

	if len(m.Tags) > 0 {
		cmd = append(cmd, "-tags", strings.Join(m.Tags, ","))
	}

	return cmd
}

type GolangConfigBuild struct {
	GolangConfigCommon

	// Output directory or filename to write resulting build artifacts to
	// Default build/${GOOS}/${GOARCH}/ if GOOS/GOARCH are set
	OutputPath string

	// Remove all filesystem paths from the resulting executable
	TrimPath bool

	paths  containerPathMapping
	target *MagnetTarget
}

// BuildContainer describes a build container image
type BuildContainer struct {
	// Name identifies the container image
	Name string
	// HostPath optionally specifies the repository path on host.
	// Defaults to working directory of the process if unspecified
	HostPath string
	// ContainerPath optionally specifies the repository path inside the container.
	// Will be computed based on host's GOPATH configuration if unspecified
	ContainerPath string
}

func (m *GolangConfigBuild) cacheDir() (path string, err error) {
	path = filepath.Join(m.target.root.AbsCacheDir(), "go")
	if err := os.MkdirAll(path, 0755); err != nil {
		return "", trace.ConvertSystemError(err)
	}
	return path, nil
}

// GolangBuild returns a builder that can be used to build a golang binary.
func (m *MagnetTarget) GolangBuild() *GolangConfigBuild {
	return &GolangConfigBuild{
		TrimPath: true,
		target:   m,
	}
}

// GolangTest returns a builder that can be used to run golang tests against a set of sources.
func (m *MagnetTarget) GolangTest() *GolangConfigTest {
	return &GolangConfigTest{
		target: m,
	}
}

// SetOutputPath sets the output directory or filename to write the resulting build artifacts to
// Default build/${GOOS}/${GOARCH}/ if GOOS/GOARCH are set.
func (m *GolangConfigBuild) SetOutputPath(path string) *GolangConfigBuild {
	m.OutputPath = path
	return m
}

// AddTag adds a build tag for the golang compiler to consider during the build.
func (m *GolangConfigBuild) AddTag(tags ...string) *GolangConfigBuild {
	m.Tags = append(m.Tags, tags...)
	return m
}

// SetMod sets the module download mode (readonly or vendor).
// Use `go help modules` for more information.
func (m *GolangConfigBuild) SetMod(mode string) *GolangConfigBuild {
	m.ModMode = mode
	return m
}

// AddLDFlag adds an ldflag to pass to the compiler.
func (m *GolangConfigBuild) AddLDFlag(flag string) *GolangConfigBuild {
	m.LDFlags = append(m.LDFlags, flag)
	return m
}

// AddLDFlags adds multiple ldflags to pass to the compiler.
func (m *GolangConfigBuild) AddLDFlags(flags []string) *GolangConfigBuild {
	m.LDFlags = append(m.LDFlags, flags...)
	return m
}

// AddGCFlag adds a flag to the go tool compile program.
func (m *GolangConfigBuild) AddGCFlag(flag string) *GolangConfigBuild {
	m.GCFlags = append(m.GCFlags, flag)
	return m
}

// AddGCCGOFlag adds a flag to pass to the gcc go compiler.
func (m *GolangConfigBuild) AddGCCGOFlag(flag string) *GolangConfigBuild {
	m.GCCGOFlags = append(m.GCCGOFlags, flag)
	return m
}

const (
	BuildModeArchive  = "archive"
	BuildModeCArchive = "c-archive"
	BuildModeCShared  = "c-shared"
	BuildModeDefault  = "default"
	BuildModeShared   = "shared"
	BuildModeExe      = "exe"
	BuildModePie      = "pie"
	BuildModePlugin   = "plugin"
)

// SetBuildMode sets the golang build mode (see go help buildmode).
func (m *GolangConfigBuild) SetBuildMode(mode string) *GolangConfigBuild {
	m.BuildMode = mode
	return m
}

// SetVerbose sets whether to pass verbose flag to go toolchain.
func (m *GolangConfigBuild) SetVerbose(v bool) *GolangConfigBuild {
	m.Verbose = v
	return m
}

// SetDryRun sets the dry-run flag on the go build toolchain.
func (m *GolangConfigBuild) SetDryRun(v bool) *GolangConfigBuild {
	m.DryRun = v
	return m
}

// SetParallelTasks allows overriding the number of parallel tasks the compiler will run (Defaults to number of cores).
func (m *GolangConfigBuild) SetParallelTasks(p int) *GolangConfigBuild {
	m.ParallelTasks = &p
	return m
}

// SetRace indicates whether to enable the race detector.
func (m *GolangConfigBuild) SetRace(b bool) *GolangConfigBuild {
	m.Race = b
	return m
}

// SetRebuild forces packages that are already up to date to be rebuilt.
func (m *GolangConfigBuild) SetRebuild(b bool) *GolangConfigBuild {
	m.Rebuild = b
	return m
}

// SetTrimpath removes filesystem paths from the resulting executable.
func (m *GolangConfigBuild) SetTrimpath(b bool) *GolangConfigBuild {
	m.TrimPath = b
	return m
}

// SetEnv sets an environment variable on the build tools.
func (m *GolangConfigBuild) SetEnv(key, value string) *GolangConfigBuild {
	if m.Env == nil {
		m.Env = make(map[string]string)
	}

	m.Env[key] = value

	return m
}

// SetEnvs allows setting multiple environment variables on the build tools.
func (m *GolangConfigBuild) SetEnvs(envs map[string]string) *GolangConfigBuild {
	if m.Env == nil {
		m.Env = make(map[string]string)
	}

	for key, value := range envs {
		m.Env[key] = value
	}

	return m
}

// SetGOOS allows overriding the GOOS env to a specific value.
func (m *GolangConfigBuild) SetGOOS(value string) *GolangConfigBuild {
	if m.Env == nil {
		m.Env = make(map[string]string)
	}
	m.Env["GOOS"] = value
	return m
}

// SetGOARCH allows overriding the default architecture for the resulting binary.
func (m *GolangConfigBuild) SetGOARCH(value string) *GolangConfigBuild {
	if m.Env == nil {
		m.Env = make(map[string]string)
	}
	m.Env["GOARCH"] = value
	return m
}

// SetBuildContainer allows specifying a docker image to use for the build. Instead of running the build toolchain
// directly, a docker container will be used to map the sources and run the build within the consistent image.
func (m *GolangConfigBuild) SetBuildContainer(value string) *GolangConfigBuild {
	m.BuildContainer = value
	return m
}

// SetBuildContainerConfig allows specifying a docker image configuration for the build. Instead of running the build toolchain
// directly, a docker container will be used to map the sources and run the build within the consistent image.
func (m *GolangConfigBuild) SetBuildContainerConfig(config BuildContainer) *GolangConfigBuild {
	m.BuildContainer = config.Name
	m.paths.hostPath = config.HostPath
	m.paths.containerPath = config.ContainerPath
	return m
}

// Build executes the build as configured.
func (m *GolangConfigBuild) Build(ctx context.Context, packages ...string) error {
	if len(m.BuildContainer) > 0 {
		return trace.Wrap(m.buildDocker(ctx, packages...))
	}

	return trace.Wrap(m.buildLocal(ctx, packages...))
}

func (m *GolangConfigBuild) buildDocker(ctx context.Context, packages ...string) (err error) {
	if err := m.paths.checkAndSetDefaults(); err != nil {
		return trace.Wrap(err)
	}

	cacheDir, err := m.cacheDir()
	if err != nil {
		return trace.Wrap(err, "failed to create build cache directory")
	}

	cmd := m.target.DockerRun().
		SetRemove(true).
		SetUID(fmt.Sprint(os.Getuid())).
		SetGID(fmt.Sprint(os.Getgid())).
		SetEnv("XDG_CACHE_HOME", "/cache").
		SetEnv("GOCACHE", "/cache/go").
		SetEnv("GOPATH", m.paths.gopath).
		SetEnvs(m.Env).
		SetWorkDir(m.paths.containerPath).
		AddVolume(DockerBindMount{
			Source:      m.paths.hostPath,
			Destination: m.paths.containerPath,
			Consistency: "delegated",
		}).
		AddVolume(DockerBindMount{
			Source:      cacheDir,
			Destination: "/cache",
			Consistency: "delegated",
		})

	gocmd := m.buildCmd(packages...)

	return trace.Wrap(cmd.Run(ctx, m.BuildContainer, gocmd[0], gocmd[1:]...))
}

func (m *GolangConfigBuild) buildLocal(ctx context.Context, packages ...string) error {
	gocmd := m.buildCmd(packages...)
	_, err := m.target.Exec().SetEnvs(m.Env).Run(ctx, gocmd[0], gocmd[1:]...)
	return trace.Wrap(err)
}

func (m *GolangConfigBuild) buildCmd(packages ...string) []string {
	cmd := append([]string{"go", "build"}, m.GolangConfigCommon.genFlags()...)

	if m.TrimPath {
		cmd = append(cmd, "-trimpath")
	}

	if len(m.OutputPath) > 0 {
		cmd = append(cmd, "-o", m.OutputPath)
	}

	return append(cmd, packages...)
}

type GolangConfigTest struct {
	GolangConfigCommon

	paths  containerPathMapping
	target *MagnetTarget
}

func (m *GolangConfigTest) cacheDir() (path string, err error) {
	path = filepath.Join(m.target.root.AbsCacheDir(), "go")
	if err := os.MkdirAll(path, 0755); err != nil {
		return "", trace.ConvertSystemError(err)
	}
	return path, nil
}

// Test executes the configured test.
func (m *GolangConfigTest) Test(ctx context.Context, packages ...string) error {
	if len(m.BuildContainer) > 0 {
		return trace.Wrap(m.testDocker(ctx, packages...))
	}

	return trace.Wrap(m.testLocal(ctx, packages...))
}

func (m *GolangConfigTest) testDocker(ctx context.Context, packages ...string) error {
	if err := m.paths.checkAndSetDefaults(); err != nil {
		return trace.Wrap(err)
	}

	cacheDir, err := m.cacheDir()
	if err != nil {
		return trace.Wrap(err, "failed to create build cache directory")
	}

	cmd := m.target.DockerRun().
		SetRemove(true).
		SetUID(fmt.Sprint(os.Getuid())).
		SetGID(fmt.Sprint(os.Getgid())).
		SetEnv("XDG_CACHE_HOME", "/cache").
		SetEnv("GOCACHE", "/cache/go").
		SetEnv("GOPATH", m.paths.gopath).
		SetEnvs(m.Env).
		SetWorkDir(m.paths.containerPath).
		AddVolume(DockerBindMount{
			Source:      m.paths.hostPath,
			Destination: m.paths.containerPath,
			Consistency: "delegated",
		}).
		AddVolume(DockerBindMount{
			Source:      cacheDir,
			Destination: "/cache",
			Consistency: "delegated",
		})

	gocmd := m.builcCmd(packages...)

	return trace.Wrap(cmd.Run(ctx, m.BuildContainer, gocmd[0], gocmd[1:]...))
}

func (m *GolangConfigTest) testLocal(ctx context.Context, packages ...string) error {
	gocmd := m.builcCmd(packages...)
	_, err := m.target.Exec().SetEnvs(m.Env).Run(ctx, gocmd[0], gocmd[1:]...)
	return trace.Wrap(err)
}

func (m *GolangConfigTest) builcCmd(packages ...string) []string {
	cmd := []string{"go", "test"}

	cmd = append(cmd, m.GolangConfigCommon.genFlags()...)

	cmd = append(cmd, packages...)

	return cmd
}

// AddTag adds a build tag for the golang compiler to consider during the build.
func (m *GolangConfigTest) AddTag(tag string) *GolangConfigTest {
	m.Tags = append(m.Tags, tag)
	return m
}

// SetMod sets the module download mode (readonly or vendor).
// Use `go help modules` for more information.
func (m *GolangConfigTest) SetMod(mode string) *GolangConfigTest {
	m.ModMode = mode
	return m
}

// AddLDFlag adds an ldflag to pass to the compiler.
func (m *GolangConfigTest) AddLDFlag(flag string) *GolangConfigTest {
	m.LDFlags = append(m.LDFlags, flag)
	return m
}

// AddLDFlags adds multiple ldflags to pass to the compiler.
func (m *GolangConfigTest) AddLDFlags(flags []string) *GolangConfigTest {
	m.LDFlags = append(m.LDFlags, flags...)
	return m
}

// AddGCFlag adds a flag to the go tool compile program.
func (m *GolangConfigTest) AddGCFlag(flag string) *GolangConfigTest {
	m.GCFlags = append(m.GCFlags, flag)
	return m
}

// AddGCCGOFlag adds a flag to pass to the gcc go compiler.
func (m *GolangConfigTest) AddGCCGOFlag(flag string) *GolangConfigTest {
	m.GCCGOFlags = append(m.GCCGOFlags, flag)
	return m
}

// SetBuildMode sets the golang build mode (see go help buildmode).
func (m *GolangConfigTest) SetBuildMode(mode string) *GolangConfigTest {
	m.BuildMode = mode
	return m
}

// SetVerbose sets whether to pass verbose flag to go toolchain.
func (m *GolangConfigTest) SetVerbose(v bool) *GolangConfigTest {
	m.Verbose = v
	return m
}

// SetDryRun sets the dry-run flag on the go build toolchain.
func (m *GolangConfigTest) SetDryRun(v bool) *GolangConfigTest {
	m.DryRun = v
	return m
}

// SetParallelTasks allows overriding the number of parallel tasks the compiler will run (Defaults to number of cores).
func (m *GolangConfigTest) SetParallelTasks(p int) *GolangConfigTest {
	m.ParallelTasks = &p
	return m
}

// SetRace indicates whether to enable the race detector.
func (m *GolangConfigTest) SetRace(b bool) *GolangConfigTest {
	m.Race = b
	return m
}

// SetRebuild forces packages that are already up to date to be rebuilt.
func (m *GolangConfigTest) SetRebuild(b bool) *GolangConfigTest {
	m.Rebuild = b
	return m
}

// SetEnv sets an environment variable on the build tools.
func (m *GolangConfigTest) SetEnv(key, value string) *GolangConfigTest {
	if m.Env == nil {
		m.Env = make(map[string]string)
	}

	m.Env[key] = value

	return m
}

// SetEnvs allows setting multiple environment variables on the build tools.
func (m *GolangConfigTest) SetEnvs(envs map[string]string) *GolangConfigTest {
	if m.Env == nil {
		m.Env = make(map[string]string)
	}

	for key, value := range envs {
		m.Env[key] = value
	}

	return m
}

// SetGOOS allows overriding the GOOS env to a specific value.
func (m *GolangConfigTest) SetGOOS(value string) *GolangConfigTest {
	if m.Env == nil {
		m.Env = make(map[string]string)
	}
	m.Env["GOOS"] = value
	return m
}

// SetGOARCH allows overriding the default architecture for the resulting binary.
func (m *GolangConfigTest) SetGOARCH(value string) *GolangConfigTest {
	if m.Env == nil {
		m.Env = make(map[string]string)
	}
	m.Env["GOARCH"] = value
	return m
}

// SetBuildContainer allows specifying a docker image to use for the build. Instead of running the build toolchain
// directly, a docker container will be used to map the sources and run the build within the consistent image.
func (m *GolangConfigTest) SetBuildContainer(value string) *GolangConfigTest {
	m.BuildContainer = value
	return m
}

// SetBuildContainerConfig allows specifying a docker image configuration for the test. Instead of running the build toolchain
// directly, a docker container will be used to map the sources and run the build within the consistent image.
func (m *GolangConfigTest) SetBuildContainerConfig(config BuildContainer) *GolangConfigTest {
	m.BuildContainer = config.Name
	m.paths.hostPath = config.HostPath
	m.paths.containerPath = config.ContainerPath
	return m
}

func (r *containerPathMapping) checkAndSetDefaults() (err error) {
	if r.hostPath == "" {
		r.hostPath, err = os.Getwd()
		if err != nil {
			return trace.Wrap(err, "failed to query working directory")
		}
	}
	// Different build containers have an inconsistent directory layout.
	// So use a distinct directory for sources
	if r.containerPath == "" {
		r.containerPath = dockerSrcPathFromGopath(r.hostPath, "/host")
		if r.containerPath != "/host" {
			r.gopath = "/host"
		}
	}
	return nil
}

type containerPathMapping struct {
	// hostPath optionally specifies the path to the package repository on host.
	// Defaults to process' working directory
	hostPath string

	// continerPath optionally specifies the path to the package repository inside
	// the container.
	// Will be computed automatically based on host's GOPATH configuration if unspecified
	containerPath string

	// gopath optionally specifies the container's GOPATH in GOPATH mode.
	// GOPATH can be overridden with GO111MODULE=on in Go 1.11+
	gopath string
}

// dockerSrcPathFromGopath builds a path for Go repository at root
// using host's GOPATH configuration.
func dockerSrcPathFromGopath(root, defaultPath string) string {
	for _, srcDir := range build.Default.SrcDirs() {
		rel, err := filepath.Rel(srcDir, root)
		// err != nil == we're not inside the current GOPATH, don't change the mount
		if err == nil {
			return filepath.Join(defaultPath, rel)
		}
	}
	return defaultPath
}
