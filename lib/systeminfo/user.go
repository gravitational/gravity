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
	"os"
	"os/exec"
	"os/user"
	"strconv"
	"strings"

	"github.com/gravitational/gravity/lib/constants"
	"github.com/gravitational/gravity/lib/defaults"
	"github.com/gravitational/gravity/lib/storage"
	"github.com/gravitational/gravity/lib/utils"

	"github.com/gravitational/trace"
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
	current, err := user.Current()
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
	usr, err := user.Lookup(name)
	if err != nil {
		return nil, trace.Wrap(convertUserError(err))
	}
	uid, err := strconv.Atoi(usr.Uid)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	gid, err := strconv.Atoi(usr.Gid)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return &User{
		Name: usr.Username,
		UID:  uid,
		GID:  gid,
	}, nil
}

// LookupUserByUID finds a user by ID
func LookupUserByUID(uid int) (*User, error) {
	usr, err := user.LookupId(strconv.Itoa(uid))
	if err != nil {
		return nil, trace.Wrap(convertUserError(err))
	}
	gid, err := strconv.Atoi(usr.Gid)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return &User{
		Name: usr.Username,
		UID:  uid,
		GID:  gid,
	}, nil
}

func checkGroup(groupName, groupID string) error {
	grp, err := user.LookupGroup(groupName)
	if err != nil {
		return trace.Wrap(convertUserError(err))
	}
	if grp.Gid != groupID {
		return trace.AlreadyExists("group %q already exists with gid %v",
			groupName, grp.Gid)
	}
	return nil
}

func checkUser(userName, userID string) error {
	usr, err := user.Lookup(userName)
	if err != nil {
		return trace.Wrap(convertUserError(err))
	}
	if usr.Uid != userID {
		return trace.AlreadyExists("user %q already exists with uid %v",
			userName, usr.Uid)
	}
	return nil
}

// NewUser creates a new user with the specified name and group.
//
// If group ID has been specified, the group is created with the given ID.
// If user ID has been specified, the user is created with the given ID
func NewUser(name, group, uid, gid string) (*User, error) {
	var args []string
	if gid != "" {
		if err := checkGroup(group, gid); err != nil && !trace.IsNotFound(err) {
			return nil, trace.Wrap(err)
		}
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
		if err := checkUser(name, uid); err != nil && !trace.IsNotFound(err) {
			return nil, trace.Wrap(err)
		}
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

// OSUser returns a new storage user from this user
func (r User) OSUser() storage.OSUser {
	return storage.OSUser{
		Name: r.Name,
		UID:  strconv.Itoa(r.UID),
		GID:  strconv.Itoa(r.GID),
	}
}

// UserFromOSUser returns a new user from the specified storage user
func UserFromOSUser(user storage.OSUser) (*User, error) {
	uid, err := strconv.Atoi(user.UID)
	if err != nil {
		return nil, trace.BadParameter("expected a numeric UID but got %v", user.UID)
	}
	gid, err := strconv.Atoi(user.GID)
	if err != nil {
		return nil, trace.BadParameter("expected a numeric GID but got %v", user.GID)
	}
	return &User{
		Name: user.Name,
		UID:  uid,
		GID:  gid,
	}, nil
}

// String returns a textual representation of this user.
// Implements fmt.Stringer
func (r User) String() string {
	return fmt.Sprintf("User(%v, uid=%v, gid=%v)", r.Name, r.UID, r.GID)
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

func convertUserError(err error) error {
	switch err.(type) {
	case user.UnknownUserError, user.UnknownUserIdError, user.UnknownGroupError, user.UnknownGroupIdError:
		return trace.NotFound(err.Error())
	}
	return err
}

func isUnrecognizedOption(msg string) bool {
	return strings.Contains(msg, "unrecognized option")
}
