package magnet

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/gravitational/trace"
)

type GolangConfigCommon struct {
	// BuildContainer is the container image to use when running Go commands
	BuildContainer string

	// GOOS to pass to the Go compiler as an env variable
	GOOS string
	// GOARCH to pass to the Go compiler as an env variable
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

	// Tags is a list of build tags to consider as satisified during the build
	Tags []string

	// Volumes lists additional docker bind mounts for container workflows
	Volumes []DockerBindMount
}

func (m *GolangConfigCommon) genFlags() []string {
	cmd := []string{}

	if len(m.LDFlags) > 0 {
		cmd = append(cmd, "-ldflags", strings.Join(m.LDFlags, " "))
	}

	if m.ParallelTasks != nil {
		cmd = append(cmd, "-p", fmt.Sprint(*m.ParallelTasks))
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
	// GOPath optionally overrides the gopath inside the container.
	//
	// If set, the module path inside the container (ContainerPath)
	// will be automatically deduced as `GOPath/src/ModulePath`.
	// The value of the attribute will be written as `GOPATH` envar
	// inside the container.
	// Also sets `GO111MODULE=auto` envar if unspecified.
	GOPath string
	// HostPath optionally specifies the repository path on host.
	// Defaults to working directory of the process if unspecified.
	HostPath string
	// ContainerPath optionally specifies the module path inside the container.
	// If unspecified and GOPath is given, will be computed as described above.
	// If unspecified and GOPath is empty, defaults to `/host`.
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
		GolangConfigCommon: GolangConfigCommon{
			Env: make(map[string]string),
		},
		target: m,
	}
}

// GolangTest returns a builder that can be used to run golang tests against a set of sources.
func (m *MagnetTarget) GolangTest() *GolangConfigTest {
	return &GolangConfigTest{
		GolangConfigCommon: GolangConfigCommon{
			Env: make(map[string]string),
		},
		target: m,
	}
}

// GolangCover returns a builder that can be used to work with the coverage tool
func (m *MagnetTarget) GolangCover() *GolangConfigCover {
	return &GolangConfigCover{
		GolangConfigCommon: GolangConfigCommon{
			Env: make(map[string]string),
		},
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

// AddVolumes adds a set of docker bind mounts to the container configuration
func (m *GolangConfigBuild) AddVolumes(volumes ...DockerBindMount) *GolangConfigBuild {
	m.Volumes = append(m.Volumes, volumes...)
	return m
}

// AddGCFlag adds a flag to the go tool compile program.
func (m *GolangConfigBuild) AddGCFlag(flag string) *GolangConfigBuild {
	m.GCFlags = append(m.GCFlags, flag)
	return m
}

// AddGCCGOFlag adds a flag to pass to the GCC Go compiler.
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

// SetBuildMode sets the Go build mode (see go help buildmode).
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
	m.Env[key] = value

	return m
}

// SetEnvs allows setting multiple environment variables on the build tools.
func (m *GolangConfigBuild) SetEnvs(envs map[string]string) *GolangConfigBuild {
	for key, value := range envs {
		m.Env[key] = value
	}

	return m
}

// SetGOOS allows overriding the GOOS env to a specific value.
func (m *GolangConfigBuild) SetGOOS(value string) *GolangConfigBuild {
	m.Env["GOOS"] = value
	return m
}

// SetGOARCH allows overriding the default architecture for the resulting binary.
func (m *GolangConfigBuild) SetGOARCH(value string) *GolangConfigBuild {
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
	m.paths.gopath = config.GOPath
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
	if err := m.paths.compute(m.target.root.ModulePath, m.Env); err != nil {
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
		}).
		AddVolume(m.Volumes...)

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

	// coverProfile optionally specifies the profile output path.
	// Configuring the coverage profile prefix will run tests with
	// coverage analysis and the results will be written to this file
	coverProfile string
	// coverProfileMode optionally specifies a coverage mode.
	// Defaults to `count`. See `go tool cover -help` for more details
	coverProfileMode string

	paths  containerPathMapping
	target *MagnetTarget
	// cacheResults controls whether the test results are cached (defaults
	// to false - e.g. no caching)
	cacheResults bool
	// count optionally specifies the number of test runs
	count int
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
	if err := m.paths.compute(m.target.root.ModulePath, m.Env); err != nil {
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
		}).
		AddVolume(m.Volumes...)

	gocmd := m.buildCmd(packages...)

	return trace.Wrap(cmd.Run(ctx, m.BuildContainer, gocmd[0], gocmd[1:]...))
}

func (m *GolangConfigTest) testLocal(ctx context.Context, packages ...string) error {
	gocmd := m.buildCmd(packages...)
	_, err := m.target.Exec().SetEnvs(m.Env).Run(ctx, gocmd[0], gocmd[1:]...)
	return trace.Wrap(err)
}

func (m *GolangConfigTest) buildCmd(packages ...string) []string {
	cmd := []string{"go", "test"}

	cmd = append(cmd, m.GolangConfigCommon.genFlags()...)
	if len(m.coverProfile) != 0 {
		cmd = append(cmd, "-cover", "-coverprofile", m.coverProfile)
		if len(m.coverProfileMode) != 0 {
			cmd = append(cmd, "-covermode", m.coverProfileMode)
		}
	}
	count := m.count
	if !m.cacheResults && count == 0 {
		count = 1
	}
	if count != 0 {
		cmd = append(cmd, "-count", fmt.Sprint(count))
	}

	return append(cmd, packages...)
}

// SetCoverProfile sets the coverage profile path.
// Configuring the coverage profile will result in the coverage profile
// file generated in the given path.
// Use `go help testflag` for more information.
func (m *GolangConfigTest) SetCoverProfile(path string) *GolangConfigTest {
	m.coverProfile = path
	return m
}

// SetCoverMode sets the coverage profile mode.
// If unspecified, defaults to `count`.
// Use `go tool cover -help` for more information.
func (m *GolangConfigTest) SetCoverMode(mode string) *GolangConfigTest {
	m.coverProfileMode = mode
	return m
}

// SetCount sets the number of test runs to execute.
// The count might be set implicitly if result caching is disabled with SetCacheResults.
func (m *GolangConfigTest) SetCount(count int) *GolangConfigTest {
	m.count = count
	return m
}

// AddTag adds a build tag for the Go compiler to consider during the build.
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

// AddVolumes adds a set of docker bind mounts to the container configuration
func (m *GolangConfigTest) AddVolumes(volumes ...DockerBindMount) *GolangConfigTest {
	m.Volumes = append(m.Volumes, volumes...)
	return m
}

// AddGCFlag adds a flag to the go tool compile program.
func (m *GolangConfigTest) AddGCFlag(flag string) *GolangConfigTest {
	m.GCFlags = append(m.GCFlags, flag)
	return m
}

// AddGCCGOFlag adds a flag to pass to the GCC Go compiler.
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
	m.Env[key] = value

	return m
}

// SetEnvs allows setting multiple environment variables on the build tools.
func (m *GolangConfigTest) SetEnvs(envs map[string]string) *GolangConfigTest {
	for key, value := range envs {
		m.Env[key] = value
	}

	return m
}

// SetGOOS allows overriding the GOOS env to a specific value.
func (m *GolangConfigTest) SetGOOS(value string) *GolangConfigTest {
	m.Env["GOOS"] = value
	return m
}

// SetGOARCH allows overriding the default architecture for the resulting binary.
func (m *GolangConfigTest) SetGOARCH(value string) *GolangConfigTest {
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
	m.paths.gopath = config.GOPath
	m.paths.hostPath = config.HostPath
	m.paths.containerPath = config.ContainerPath
	return m
}

// SetCacheResults controls whether the test uses previously cached
// results
func (m *GolangConfigTest) SetCacheResults(cacheResults bool) *GolangConfigTest {
	m.cacheResults = cacheResults
	return m
}

// GolangConfigCover manages an invocation of the Go coverage tool
type GolangConfigCover struct {
	GolangConfigCommon

	// profilePath specifies the path to the coverage profile
	// previously generated with `go test`
	profilePath string
	// mode optionally specifies the coverage profile mode.
	// If unspecified, defaults to `count`
	mode string
	// outputPath optionally specifies the path to the output HTML
	outputPath string
	paths      containerPathMapping
	target     *MagnetTarget
}

// SetBuildContainerConfig allows specifying a docker image configuration for the build. Instead of running the build toolchain
// directly, a docker container will be used to map the sources and run the tool within the consistent image.
func (m *GolangConfigCover) SetBuildContainerConfig(config BuildContainer) *GolangConfigCover {
	m.BuildContainer = config.Name
	m.paths.gopath = config.GOPath
	m.paths.hostPath = config.HostPath
	m.paths.containerPath = config.ContainerPath
	return m
}

// SetProfile sets the path to the cover profile previously generated
// with 'go test'
func (m *GolangConfigCover) SetProfile(path string) *GolangConfigCover {
	m.profilePath = path
	return m
}

// SetOutput sets the path to the output HTML coverage report as generated by Run
func (m *GolangConfigCover) SetOutput(path string) *GolangConfigCover {
	m.outputPath = path
	return m
}

// SetMode sets the coverage profile mode.
// If unspecified, defaults to `count`.
// Use `go tool cover -help` for more information.
func (m *GolangConfigCover) SetMode(mode string) *GolangConfigCover {
	m.mode = mode
	return m
}

// SetEnv sets an environment variable on the tool.
func (m *GolangConfigCover) SetEnv(key, value string) *GolangConfigCover {
	m.Env[key] = value
	return m
}

// SetEnvs allows setting multiple environment variables on the tool.
func (m *GolangConfigCover) SetEnvs(envs map[string]string) *GolangConfigCover {
	for key, value := range envs {
		m.Env[key] = value
	}
	return m
}

// SetMod sets the module download mode (readonly or vendor).
// Use `go help modules` for more information.
func (m *GolangConfigCover) SetMod(mode string) *GolangConfigCover {
	m.ModMode = mode
	return m
}

// Run runs the coverage tool
func (m *GolangConfigCover) Run(ctx context.Context) error {
	if len(m.BuildContainer) > 0 {
		return trace.Wrap(m.docker(ctx))
	}
	return trace.Wrap(m.local(ctx))
}

// docker runs the coverage analysis inside a docker container
func (m *GolangConfigCover) docker(ctx context.Context) error {
	if err := m.paths.compute(m.target.root.ModulePath, m.Env); err != nil {
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
		}).
		AddVolume(m.Volumes...)
	gocmd := m.toolCmd()
	return trace.Wrap(cmd.Run(ctx, m.BuildContainer, gocmd[0], gocmd[1:]...))
}

// local runs the coverage analysis directly on host
func (m *GolangConfigCover) local(ctx context.Context) error {
	gocmd := m.toolCmd()
	_, err := m.target.Exec().SetEnvs(m.Env).Run(ctx, gocmd[0], gocmd[1:]...)
	return trace.Wrap(err)
}

func (m *GolangConfigCover) toolCmd() []string {
	cmd := []string{"go", "tool", "cover"}
	if len(m.outputPath) > 0 {
		cmd = append(cmd, "-html", m.profilePath, "-o", m.outputPath)
	} else {
		cmd = append(cmd, "-func", m.profilePath)
	}
	if len(m.mode) > 0 {
		cmd = append(cmd, "-mode", m.mode)
	}
	return cmd
}

func (m *GolangConfigCover) cacheDir() (path string, err error) {
	path = filepath.Join(m.target.root.AbsCacheDir(), "go")
	if err := os.MkdirAll(path, 0755); err != nil {
		return "", trace.ConvertSystemError(err)
	}
	return path, nil
}

func (r *containerPathMapping) compute(modulePath string, env map[string]string) (err error) {
	if r.hostPath == "" {
		r.hostPath, err = os.Getwd()
		if err != nil {
			return trace.Wrap(err, "failed to query working directory")
		}
	}
	// Different build containers have an inconsistent directory layout.
	// So use a distinct directory for sources
	if r.containerPath == "" {
		if r.gopath != "" {
			r.containerPath = filepath.Join(r.gopath, "src", modulePath)
			env["GOPATH"] = r.gopath
			if _, ok := env["GO111MODULE"]; !ok {
				env["GO111MODULE"] = "auto"
			}
		} else {
			delete(env, "GOPATH")
			r.containerPath = "/host"
		}
	}
	return nil
}

type containerPathMapping struct {
	// hostPath optionally specifies the path to the package repository on host.
	// Defaults to process' working directory
	hostPath string

	// containerPath optionally specifies the path to the package repository inside
	// the container.
	// Will be computed automatically based on host's GOPATH configuration if unspecified
	containerPath string

	// gopath optionally overrides the container's GOPATH in GOPATH mode.
	gopath string
}
