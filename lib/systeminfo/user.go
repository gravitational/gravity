/*
Copyright 2018 Gravitational, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package systeminfo

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"os/exec"
	osuser "os/user"
	"strconv"
	"strings"

	"github.com/gravitational/gravity/lib/constants"
	"github.com/gravitational/gravity/lib/defaults"
	"github.com/gravitational/gravity/lib/utils"

	"github.com/gravitational/trace"
	"github.com/opencontainers/runc/libcontainer/user"
	log "github.com/sirupsen/logrus"
)

// GetRealUser returns a name of the current user
func GetRealUser() (*User, error) {
	if os.Getenv(constants.EnvSudoUser) != "" {
		uidS := os.Getenv(constants.EnvSudoUID)
		uid, err := strconv.Atoi(uidS)
		if err != nil {
			return nil, trace.Wrap(err, "invalid sudo user ID: %v", uidS)
		}
		gidS := os.Getenv(constants.EnvSudoGID)
		gid, err := strconv.Atoi(gidS)
		if err != nil {
			return nil, trace.Wrap(err, "invalid sudo user group ID: %v", gidS)
		}
		return &User{
			Name: os.Getenv(constants.EnvSudoUser),
			UID:  uid,
			GID:  gid,
		}, nil
	}
	current, err := osuser.Current()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	uid, err := strconv.Atoi(current.Uid)
	if err != nil {
		return nil, trace.Wrap(err, "invalid user ID: %v", current.Uid)
	}
	gid, err := strconv.Atoi(current.Gid)
	if err != nil {
		return nil, trace.Wrap(err, "invalid user group ID: %v", current.Gid)
	}
	return &User{
		Name: current.Username,
		UID:  uid,
		GID:  gid,
	}, nil
}

// LookupUserByName finds a user by name
func LookupUserByName(name string) (*User, error) {
	u, err := lookupUser(func(u user.User) bool {
		return u.Name == name
	})
	if err != nil {
		return nil, trace.Wrap(convertError(err.Error()))
	}

	return &User{
		Name: u.Name,
		UID:  u.Uid,
		GID:  u.Gid,
	}, nil
}

// LookupUserByUID finds a user by ID
func LookupUserByUID(uid int) (*User, error) {
	u, err := lookupUser(func(u user.User) bool {
		return u.Uid == uid
	})
	if err != nil {
		return nil, trace.Wrap(convertError(err.Error()))
	}

	return &User{
		Name: u.Name,
		UID:  u.Uid,
		GID:  u.Gid,
	}, nil
}

// NewUser creates a new user with the specified name and group.
//
// If group ID has been specified, the group is created with the given ID.
// If user ID has been specified, the user is created with the given ID
func NewUser(name, group, uid, gid string) (*User, error) {
	var args []string
	if gid != "" {
		args = append(args, "--gid", gid)
	}
	output, err := runCmd(groupAddCommand(group, args...))
	if err != nil && !trace.IsAlreadyExists(err) {
		log.Warnf("Failed to create group %q in regular groups database: %v %s.", group, trace.DebugReport(err), output)
		extraOutput, extraErr := runCmd(groupAddCommand(name, append(args, "--extrausers")...))
		if extraErr != nil && !isUnrecognizedOption(string(extraOutput)) {
			return nil, trace.NewAggregate(
				trace.Wrap(err, "failed to create group %q in regular groups database: %s", group, output),
				trace.Wrap(extraErr, "failed to create group %q in extrausers database: %s", group, extraOutput))
		}
		log.Debugf("Group %q created in extrausers database.", group)
	}

	if gid == "" {
		gid = group
	}
	args = []string{"--gid", gid}
	if uid != "" {
		args = append(args, "--uid", uid)
	}
	output, err = runCmd(userAddCommand(name, args...))
	if err != nil && !trace.IsAlreadyExists(err) {
		log.Warnf("Failed to create user %q in regular users database: %v %s.", name, trace.DebugReport(err), output)
		extraOutput, extraErr := runCmd(userAddCommand(name, append(args, "--extrausers")...))
		if extraErr != nil && !isUnrecognizedOption(string(extraOutput)) {
			return nil, trace.NewAggregate(
				trace.Wrap(err, "failed to create user %q in regular users database: %s", name, output),
				trace.Wrap(extraErr, "failed to create user %q in extrausers database: %s", name, extraOutput))
		}
		log.Debugf("User %q created in extrausers database.", name)
	}

	user, err := LookupUserByName(name)
	if err != nil {
		return nil, trace.Wrap(err, "failed to create user %q/group %q", name, group)
	}

	return user, nil
}

// DefaultServiceUser returns the default service user
func DefaultServiceUser() *User {
	return &User{
		Name: defaults.ServiceUser,
		UID:  defaults.ServiceUID,
		GID:  defaults.ServiceGID,
	}
}

// String returns a textual representation of this user.
// Implements fmt.Stringer
func (r User) String() string {
	return fmt.Sprintf("user(%q, uid=%v, gid=%v)", r.Name, r.UID, r.GID)
}

// User describes a system user
type User struct {
	// Name is the user name
	Name string `json:"name"`
	// UID is user id
	UID int `json:"uid"`
	// GID is group id
	GID int `json:"gid"`
}

// runCmd runs the command cmd and returns the output.
func runCmd(cmd *exec.Cmd) ([]byte, error) {
	var out bytes.Buffer
	err := utils.ExecL(cmd, &out, log.StandardLogger())
	if err != nil {
		return bytes.TrimSpace(out.Bytes()), trace.Wrap(convertError(out.String()))
	}
	return out.Bytes(), nil
}

func userAddCommand(name string, extraArgs ...string) *exec.Cmd {
	args := []string{"--system", "--no-create-home"}
	args = append(args, name)
	cmd := exec.Command("/usr/sbin/useradd", append(args, extraArgs...)...)
	return cmd
}

func groupAddCommand(name string, extraArgs ...string) *exec.Cmd {
	args := []string{"--system"}
	args = append(args, name)
	cmd := exec.Command("/usr/sbin/groupadd", append(args, extraArgs...)...)
	return cmd
}

func convertError(output string) error {
	switch {
	case strings.Contains(output, "no matching entries"):
		return trace.NotFound(output)
	case strings.Contains(output, "already exists"):
		return trace.AlreadyExists(output)
	default:
		return trace.BadParameter(output)
	}
}

func isUnrecognizedOption(msg string) bool {
	return strings.Contains(msg, "unrecognized option")
}

func lookupUser(filter func(u user.User) bool) (*user.User, error) {
	// Get operating system-specific passwd reader.
	passwd, err := getPasswdUbuntuCore()
	if err != nil && !trace.IsNotFound(err) {
		return nil, trace.Wrap(err)
	}
	if trace.IsNotFound(err) {
		passwd, err = user.GetPasswd()
		if err != nil {
			return nil, trace.ConvertSystemError(err)
		}
	}
	defer passwd.Close()

	// Get the users.
	users, err := user.ParsePasswdFilter(passwd, filter)
	if err != nil {
		return nil, trace.ConvertSystemError(err)
	}

	if len(users) == 0 {
		return nil, trace.NotFound("no matching entries in passwd file")
	}

	// Assume the first entry is the "correct" one.
	return &users[0], nil
}

func lookupGroup(filter func(g user.Group) bool) (*user.Group, error) {
	// Get operating system-specific group reader.
	group, err := getGroupUbuntuCore()
	if err != nil && !trace.IsNotFound(err) {
		return nil, trace.Wrap(err)
	}
	if trace.IsNotFound(err) {
		group, err = user.GetGroup()
		if err != nil {
			return nil, trace.ConvertSystemError(err)
		}
	}
	defer group.Close()

	// Get the group.
	groups, err := user.ParseGroupFilter(group, filter)
	if err != nil {
		return nil, trace.ConvertSystemError(err)
	}

	if len(groups) == 0 {
		return nil, trace.NotFound("no matching entries in group file")
	}

	// Assume the first entry is the "correct" one.
	return &groups[0], nil
}

func getPasswdUbuntuCore() (io.ReadCloser, error) {
	f, err := os.Open(ubuntuCorePasswdPath)
	if err != nil {
		return nil, trace.ConvertSystemError(err)
	}
	return f, nil
}

func getGroupUbuntuCore() (io.ReadCloser, error) {
	f, err := os.Open(ubuntuCoreGroupPath)
	if err != nil {
		return nil, trace.ConvertSystemError(err)
	}
	return f, nil
}

// Ubuntu-Core-specific path to the passwd and group formatted files.
const (
	ubuntuCorePasswdPath = "/var/lib/extrausers/passwd"
	ubuntuCoreGroupPath  = "/var/lib/extrausers/group"
)
