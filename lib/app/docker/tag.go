package docker

import (
	"fmt"
	"strings"
)

// TagSpec is a helper to deal with Docker 'registry tag' format like
// host:port/name:version
type TagSpec struct {
	Name    string // same as 'repo' in dockerspeak, could be something like 'vendor.com/app'
	Version string // same as 'tag' in dockerspeak, usually '1.2.3' or 'latest'
}

// TagFromString creates a new tag structure from string. It never fails,
// but you can use tag.IsValid() method later.
func TagFromString(s string) (t TagSpec) {
	s = strings.TrimSpace(s)
	idx := strings.LastIndex(s, ":")
	if idx > 0 {
		t.Name = s[:idx]
		t.Version = s[idx+1:]
	} else {
		t.Name = s
		t.Version = "latest"
	}
	return t
}

// IsValid returns 'true' if it is a valid tag
func (t TagSpec) IsValid() bool {
	return len(t.Name) > 0 && len(t.Version) > 0
}

// String returns a long/full name of the tag
func (t TagSpec) String() string {
	if !t.IsValid() {
		return ""
	}
	return fmt.Sprintf("%s:%s", t.Name, t.Version)
}

func (t TagSpec) Equals(tag string) bool {
	other := TagFromString(tag)
	return t.Name == other.Name && t.Version == other.Version
}
