// +build !selinux_embed

package selinux

import (
	"net/http"
)

// Policy contains the SELinux policy.
var Policy http.FileSystem = http.Dir("assets")
