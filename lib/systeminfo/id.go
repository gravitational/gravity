package systeminfo

import (
	"bufio"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"regexp"
	"strconv"
	"strings"

	"github.com/gravitational/gravity/lib/storage"
	"github.com/gravitational/gravity/lib/utils"

	"github.com/gravitational/trace"
	log "github.com/sirupsen/logrus"
)

const (
	// RedHat identifies a RedHat Enterprise Linux system or one of its descent
	RedHat = "rhel"

	releaseMetadataPath = "/etc/os-release"
	releaseInfoPath     = "/etc/system-release"
)

// OSInfo obtains identification information for the host operating system
func OSInfo() (info *OS, err error) {
	file, err := os.Open(releaseMetadataPath)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	defer file.Close()
	return getInfo(file)
}

func getInfo(r io.Reader) (info *OS, err error) {
	s := bufio.NewScanner(r)
	s.Split(bufio.ScanLines)
	info = &OS{}
	for s.Scan() {
		line := strings.TrimSpace(s.Text())
		if len(line) == 0 {
			continue
		}
		parts := strings.SplitN(line, "=", 2)
		var value string
		if value, err = strconv.Unquote(parts[1]); err != nil {
			value = parts[1]
		}
		switch parts[0] {
		case "ID":
			info.ID = value
		case "ID_LIKE":
			info.Like = strings.Split(value, " ")
		case "VERSION_ID":
			info.Version = value
		}
	}

	if err := s.Err(); err != nil {
		return nil, trace.Wrap(err)
	}

	if !info.IsRedHat() {
		return info, nil
	}

	// Handle redhat-descending OS
	content, err := ioutil.ReadFile(releaseInfoPath)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	releaseVersion := getReleaseVersion(string(content))
	if releaseVersion == "" {
		log.Debugf("Unable to parse OS release version from file %s with content: %s", releaseInfoPath, content)
	} else {
		if strings.HasPrefix(releaseVersion, info.Version) {
			info.Version = releaseVersion
		}
	}
	return info, nil
}

func getReleaseVersion(s string) string {
	re := regexp.MustCompile(".*?([0-9\\.]+).*")
	result := re.FindStringSubmatch(s)
	if len(result) == 0 {
		return ""
	}
	return result[1]
}

// OS aliases operating system info
type OS storage.OSInfo

// IsRedHat determines if this info refers to a RedHat system or one of its descent
func (r OS) IsRedHat() bool {
	return r.ID == RedHat || utils.StringInSlice(r.Like, RedHat)
}

// Name returns a name/version for this OS info, e.g. "centos 7.1"
func (r OS) Name() string {
	return fmt.Sprintf("%v %v", r.ID, r.Version)
}
