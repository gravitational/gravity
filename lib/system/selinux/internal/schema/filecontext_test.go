/*
Copyright 2020 Gravitational, Inc.

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

package schema

import (
	"strings"

	. "gopkg.in/check.v1"
)

func (*S) TestParsesFcontextFile(c *C) {
	items, err := ParseFcontextFile(strings.NewReader(fcontext))
	c.Assert(err, IsNil)
	c.Assert(items, DeepEquals, []FcontextFileItem{
		{
			Path:     "/var/lib/gravity(/.*)?",
			FileType: AllFiles,
			Label:    withType("gravity_home_t"),
		},
		{
			Path:     "/usr/bin/gravity",
			FileType: RegularFile,
			Label:    withType("gravity_exec_t"),
		},
		{
			Path:     `HOME_DIR/\.my\.cnf`,
			FileType: RegularFile,
			Label:    withType("mysqld_home_t"),
		},
		{
			Path:     "/dev/[shmxv]d[^/]*",
			FileType: BlockDevice,
			Label:    withType("fixed_disk_device_t"),
		},
		{
			Path:     "/dev/[pt]ty[a-ep-z][0-9a-f]",
			FileType: CharDevice,
			Label:    withType("bsdpty_device_t"),
		},
		{
			Path:     "/dev/xen/tapctrl.*",
			FileType: NamedPipe,
			Label:    withType("xenctl_t"),
		},
		{
			Path:     "/var/run/glusterd.*",
			FileType: Socket,
			Label:    withType("glusterd_var_run_t"),
		},
		{
			Path:     "/etc/localtime",
			FileType: Symlink,
			Label:    withType("locale_t"),
		},
		{
			Path:     `/usr/lost\+found`,
			FileType: Directory,
			Label:    withType("lost_found_t"),
		},
	})
}

func withType(typ string) *Label {
	return &Label{
		User:          "system_u",
		Role:          "object_r",
		Type:          typ,
		SecurityRange: "s0",
	}
}

const fcontext = `

# any file
/var/lib/gravity(/.*)?		gen_context(system_u:object_r:gravity_home_t,s0)
# regular file
/usr/bin/gravity	--	gen_context(system_u:object_r:gravity_exec_t,s0)
# using HOME_DIR placeholder
HOME_DIR/\.my\.cnf	--	gen_context(system_u:object_r:mysqld_home_t, s0)
# block device
/dev/[shmxv]d[^/]*	-b gen_context(system_u:object_r:fixed_disk_device_t,s0)
# character device
/dev/[pt]ty[a-ep-z][0-9a-f] -c	 gen_context(system_u:object_r:bsdpty_device_t,s0)
# named pipe
/dev/xen/tapctrl.*                                 -p gen_context(system_u:object_r:xenctl_t, 	s0)
# socket
/var/run/glusterd.*  -s gen_context(system_u:object_r:glusterd_var_run_t,s0)
# symbolic link
/etc/localtime -l	gen_context(system_u:object_r:locale_t,s0)
# directory
/usr/lost\+found	-d gen_context(system_u:object_r:lost_found_t,s0)

`
