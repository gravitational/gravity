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
	// BuildContainer is the container image to use when running go commands
	BuildContainer string

	// GOOS to pass to the go compiler as an env variable
	GOOS string
	// GOARCH to pass to the go compiler as an env variable
	GOARCH string

	// Env is a set of environment variables to pass to the compiler
	Env map[string]string

	// Rebuilds forces rebuilding of packages that are already up todate
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

	magnet *Magnet
}

func (m *Magnet) GolangBuild() *GolangConfigBuild {
	return &GolangConfigBuild{
		TrimPath: true,
		magnet:   m,
	}
}

func (m *Magnet) GolangTest() *GolangConfigTest {
	return &GolangConfigTest{
		magnet: m,
	}
}

func (m *GolangConfigBuild) SetOutputPath(path string) *GolangConfigBuild {
	m.OutputPath = path
	return m
}

func (m *GolangConfigBuild) AddTag(tag string) *GolangConfigBuild {
	m.Tags = append(m.Tags, tag)
	return m
}

func (m *GolangConfigBuild) SetMod(mode string) *GolangConfigBuild {
	m.ModMode = mode
	return m
}

func (m *GolangConfigBuild) AddLDFlag(flag string) *GolangConfigBuild {
	m.LDFlags = append(m.LDFlags, flag)
	return m
}

func (m *GolangConfigBuild) AddLDFlags(flags []string) *GolangConfigBuild {
	m.LDFlags = append(m.LDFlags, flags...)
	return m
}

func (m *GolangConfigBuild) AddGCFlag(flag string) *GolangConfigBuild {
	m.GCFlags = append(m.GCFlags, flag)
	return m
}

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

func (m *GolangConfigBuild) SetBuildMode(mode string) *GolangConfigBuild {
	m.BuildMode = mode
	return m
}

func (m *GolangConfigBuild) SetVerbose(v bool) *GolangConfigBuild {
	m.Verbose = v
	return m
}

func (m *GolangConfigBuild) SetDryRun(v bool) *GolangConfigBuild {
	m.DryRun = v
	return m
}

func (m *GolangConfigBuild) SetParallelTasks(p int) *GolangConfigBuild {
	m.ParallelTasks = &p
	return m
}

func (m *GolangConfigBuild) SetRace(b bool) *GolangConfigBuild {
	m.Race = b
	return m
}

func (m *GolangConfigBuild) SetRebuild(b bool) *GolangConfigBuild {
	m.Rebuild = b
	return m
}

func (m *GolangConfigBuild) SetTrimpath(b bool) *GolangConfigBuild {
	m.TrimPath = b
	return m
}

func (m *GolangConfigBuild) SetEnv(key, value string) *GolangConfigBuild {
	if m.Env == nil {
		m.Env = make(map[string]string)
	}

	m.Env[key] = value

	return m
}

func (m *GolangConfigBuild) SetEnvs(envs map[string]string) *GolangConfigBuild {
	if m.Env == nil {
		m.Env = make(map[string]string)
	}

	for key, value := range envs {
		m.Env[key] = value
	}

	return m
}

func (m *GolangConfigBuild) SetGOOS(value string) *GolangConfigBuild {
	if m.Env == nil {
		m.Env = make(map[string]string)
	}
	m.Env["GOOS"] = value
	return m
}

func (m *GolangConfigBuild) SetGOARCH(value string) *GolangConfigBuild {
	if m.Env == nil {
		m.Env = make(map[string]string)
	}
	m.Env["GOARCH"] = value
	return m
}

func (m *GolangConfigBuild) SetBuildContainer(value string) *GolangConfigBuild {
	m.BuildContainer = value
	return m
}

func (m *GolangConfigBuild) Build(ctx context.Context, packages ...string) error {
	if len(m.BuildContainer) > 0 {
		return trace.Wrap(m.buildDocker(ctx, packages...))
	}

	return trace.Wrap(m.buildLocal(ctx, packages...))
}

func (m *GolangConfigBuild) buildDocker(ctx context.Context, packages ...string) error {
	wd, err := os.Getwd()
	if err != nil {
		panic(err)
	}

	wdTarget := "/gopath"

	gopath := os.Getenv("GOPATH")
	if gopath != "" {
		rel, err := filepath.Rel(gopath, wd)
		// err == we're not inside the current GOPATH, don't change the mount
		if err == nil {
			wdTarget = filepath.Join("/gopath", rel)
		}
	}

	cmd := m.magnet.DockerRun().
		SetRemove(true).
		SetUID(fmt.Sprint(os.Getuid())).
		SetGID(fmt.Sprint(os.Getgid())).
		SetEnv("XDG_CACHE_HOME", filepath.Join(wdTarget, "build/cache")).
		SetEnv("GOCACHE", filepath.Join(wdTarget, "build/cache/go")).
		SetEnvs(m.Env).
		AddVolume(fmt.Sprint(wd, ":", wdTarget, ":delegated")).SetWorkDir(wdTarget)

	gocmd := m.builcCmd(packages...)

	return trace.Wrap(cmd.Run(ctx, m.BuildContainer, gocmd[0], gocmd[1:]...))
}

func (m *GolangConfigBuild) buildLocal(ctx context.Context, packages ...string) error {
	gocmd := m.builcCmd(packages...)
	_, err := m.magnet.Exec().SetEnvs(m.Env).Run(ctx, gocmd[0], gocmd[1:]...)
	return trace.Wrap(err)
}

func (m *GolangConfigBuild) builcCmd(packages ...string) []string {
	cmd := []string{"go", "build"}

	cmd = append(cmd, m.GolangConfigCommon.genFlags()...)

	if len(m.OutputPath) > 0 {
		cmd = append(cmd, "-o", m.OutputPath)
	}

	if m.TrimPath {
		cmd = append(cmd, "-trimpath")
	}

	if len(m.OutputPath) > 0 {
		cmd = append(cmd, "-o", m.OutputPath)
	} else if len(m.Env["GOOS"]) > 0 && len(m.Env["GOARCH"]) > 0 {
		if len(packages) == 1 {
			cmd = append(cmd, "-o", fmt.Sprintf("build/%v_%v/%v", m.Env["GOOS"], m.Env["GOARCH"], filepath.Base(packages[0])))
		} else {
			buildDir := fmt.Sprintf("build/%v_%v/", m.Env["GOOS"], m.Env["GOARCH"])
			cmd = append(cmd, "-o", buildDir)
			_ = os.MkdirAll(buildDir, 0755)
		}
	}

	cmd = append(cmd, packages...)

	return cmd
}

type GolangConfigTest struct {
	GolangConfigCommon

	magnet *Magnet
}

func (m *GolangConfigTest) Test(ctx context.Context, packages ...string) error {
	if len(m.BuildContainer) > 0 {
		return trace.Wrap(m.testDocker(ctx, packages...))
	}

	return trace.Wrap(m.testLocal(ctx, packages...))
}

func (m *GolangConfigTest) testDocker(ctx context.Context, packages ...string) error {
	wd, err := os.Getwd()
	if err != nil {
		panic(err)
	}

	wdTarget := "/gopath"

	gopath := os.Getenv("GOPATH")
	if gopath != "" {
		rel, err := filepath.Rel(gopath, wd)
		// err == we're not inside the current GOPATH, don't change the mount
		if err == nil {
			wdTarget = filepath.Join("/gopath", rel)
		}
	}

	cmd := m.magnet.DockerRun().
		SetRemove(true).
		SetUID(fmt.Sprint(os.Getuid())).
		SetGID(fmt.Sprint(os.Getgid())).
		SetEnv("XDG_CACHE_HOME", filepath.Join(wdTarget, "build/cache")).
		SetEnv("GOCACHE", filepath.Join(wdTarget, "build/cache/go")).
		SetEnvs(m.Env).
		AddVolume(fmt.Sprint(wd, ":", wdTarget, ":delegated")).SetWorkDir(wdTarget)

	gocmd := m.builcCmd(packages...)

	return trace.Wrap(cmd.Run(ctx, m.BuildContainer, gocmd[0], gocmd[1:]...))
}

func (m *GolangConfigTest) testLocal(ctx context.Context, packages ...string) error {
	gocmd := m.builcCmd(packages...)
	_, err := m.magnet.Exec().SetEnvs(m.Env).Run(ctx, gocmd[0], gocmd[1:]...)
	return trace.Wrap(err)
}

func (m *GolangConfigTest) builcCmd(packages ...string) []string {
	cmd := []string{"go", "test"}

	cmd = append(cmd, m.GolangConfigCommon.genFlags()...)

	cmd = append(cmd, packages...)

	return cmd
}

func (m *GolangConfigTest) AddTag(tag string) *GolangConfigTest {
	m.Tags = append(m.Tags, tag)
	return m
}

func (m *GolangConfigTest) SetMod(mode string) *GolangConfigTest {
	m.ModMode = mode
	return m
}

func (m *GolangConfigTest) AddLDFlag(flag string) *GolangConfigTest {
	m.LDFlags = append(m.LDFlags, flag)
	return m
}

func (m *GolangConfigTest) AddLDFlags(flags []string) *GolangConfigTest {
	m.LDFlags = append(m.LDFlags, flags...)
	return m
}

func (m *GolangConfigTest) AddGCFlag(flag string) *GolangConfigTest {
	m.GCFlags = append(m.GCFlags, flag)
	return m
}

func (m *GolangConfigTest) AddGCCGOFlag(flag string) *GolangConfigTest {
	m.GCCGOFlags = append(m.GCCGOFlags, flag)
	return m
}

func (m *GolangConfigTest) SetBuildMode(mode string) *GolangConfigTest {
	m.BuildMode = mode
	return m
}

func (m *GolangConfigTest) SetVerbose(v bool) *GolangConfigTest {
	m.Verbose = v
	return m
}

func (m *GolangConfigTest) SetDryRun(v bool) *GolangConfigTest {
	m.DryRun = v
	return m
}

func (m *GolangConfigTest) SetParallelTasks(p int) *GolangConfigTest {
	m.ParallelTasks = &p
	return m
}

func (m *GolangConfigTest) SetRace(b bool) *GolangConfigTest {
	m.Race = b
	return m
}

func (m *GolangConfigTest) SetRebuild(b bool) *GolangConfigTest {
	m.Rebuild = b
	return m
}

func (m *GolangConfigTest) SetEnv(key, value string) *GolangConfigTest {
	if m.Env == nil {
		m.Env = make(map[string]string)
	}

	m.Env[key] = value

	return m
}

func (m *GolangConfigTest) SetEnvs(envs map[string]string) *GolangConfigTest {
	if m.Env == nil {
		m.Env = make(map[string]string)
	}

	for key, value := range envs {
		m.Env[key] = value
	}

	return m
}

func (m *GolangConfigTest) SetGOOS(value string) *GolangConfigTest {
	if m.Env == nil {
		m.Env = make(map[string]string)
	}
	m.Env["GOOS"] = value
	return m
}

func (m *GolangConfigTest) SetGOARCH(value string) *GolangConfigTest {
	if m.Env == nil {
		m.Env = make(map[string]string)
	}
	m.Env["GOARCH"] = value
	return m
}

func (m *GolangConfigTest) SetBuildContainer(value string) *GolangConfigTest {
	m.BuildContainer = value
	return m
}
