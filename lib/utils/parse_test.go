package utils

import (
	"strings"

	"github.com/gravitational/gravity/lib/compare"

	"gopkg.in/check.v1"
)

type ParseSuite struct{}

var _ = check.Suite(&ParseSuite{})

func (s *ParseSuite) TestParsePorts(c *check.C) {
	type testCase struct {
		in       string
		outPorts []int
		outError bool
	}
	testCases := []testCase{
		{
			in:       "80, 8080, 8000-8002",
			outPorts: []int{80, 8080, 8000, 8001, 8002},
		},
		{
			in:       "80, a-b",
			outError: true,
		},
	}
	for _, tc := range testCases {
		out, err := ParsePorts(tc.in)
		if tc.outError {
			c.Assert(err, check.NotNil)
			c.Assert(out, check.IsNil)
		} else {
			c.Assert(err, check.IsNil)
			c.Assert(out, check.DeepEquals, tc.outPorts)
		}
	}
}

func (s *ParseSuite) TestParseDDOutput(c *check.C) {
	type testCase struct {
		in      string
		out     uint64
		comment string
	}
	testCases := []testCase{
		{
			in: `1+0 records in
1+0 records out
1073741824 bytes (1.1 GB) copied, 4.52455 s, 237 MB/s`,
			out:     237000000,
			comment: "parses speed value as an integer",
		},
		{
			in: `1024+0 records in
1024+0 records out
10485760 bytes (10 MB) copied, 0.00883009 s, 1.2 GB/s`,
			out:     1200000000,
			comment: "parses speed value as a float",
		},
		{
			in: `1+0 records in
1+0 records out
1024 bytes (1,0 kB, 1,0 KiB) copied, 7,2817e-05 s, 14,1 MB/s`,
			out:     14100000,
			comment: "parses speed value as comma-formatted float",
		},
	}
	for _, tc := range testCases {
		out, err := ParseDDOutput(tc.in)
		c.Assert(err, check.IsNil)
		c.Assert(out, check.Equals, tc.out, check.Commentf(tc.comment))
	}
}

func (*ParseSuite) TestUserParsePasswd(c *check.C) {
	users, err := ParsePasswd(strings.NewReader(`
root:x:0:0:root:/root:/bin/bash
adm:x:3:4:adm:/var/adm:/bin/false
this is just some garbage data
`))

	c.Assert(err, check.IsNil)
	compare.DeepCompare(c, users, []User{
		{Name: "root", Pass: "x", Gecos: "root", Home: "/root", Shell: "/bin/bash"},
		{Name: "adm", Pass: "x", Uid: 3, Gid: 4, Gecos: "adm", Home: "/var/adm", Shell: "/bin/false"},
		{Name: "this is just some garbage data"},
	})
}

func (s *ParseSuite) TestParseSystemdVersion(c *check.C) {
	out := "systemd 228\n+PAM -AUDIT +SELINUX -IMA +APPARMOR -SMACK +SYSVINIT +UTMP +LIBCRYPTSETUP +GCRYPT -GNUTLS +ACL +XZ -LZ4 +SECCOMP +BLKID -ELFUTILS +KMOD -IDN\n"
	version, err := ParseSystemdVersion(out)
	c.Assert(err, check.IsNil)
	c.Assert(version, check.Equals, 228)
}

func (s ParseSuite) TestParseHostOverride(c *check.C) {
	type testCase struct {
		in        string
		outDomain string
		outIP     string
		outError  bool
		comment   check.CommentInterface
	}
	testCases := []testCase{
		{
			in:        "example.com/10.0.0.1",
			outDomain: "example.com",
			outIP:     "10.0.0.1",
			comment:   check.Commentf("Correct host override."),
		},
		{
			in:       "example.com:10.0.0.1",
			outError: true,
			comment:  check.Commentf("Incorrectly formatted host override."),
		},
		{
			in:       "exa~mple.com/10.0.0.1",
			outError: true,
			comment:  check.Commentf("Invalid domain name."),
		},
		{
			in:       "example.com/foo.com",
			outError: true,
			comment:  check.Commentf("Invalid IP address."),
		},
	}
	for _, testCase := range testCases {
		domain, ip, err := ParseHostOverride(testCase.in)
		c.Assert(domain, check.Equals, testCase.outDomain, testCase.comment)
		c.Assert(ip, check.Equals, testCase.outIP, testCase.comment)
		if testCase.outError {
			c.Assert(err, check.NotNil, testCase.comment)
		} else {
			c.Assert(err, check.IsNil, testCase.comment)
		}
	}
}

func (s ParseSuite) TestParseZoneOverride(c *check.C) {
	type testCase struct {
		in       string
		outZone  string
		outNs    string
		outError bool
		comment  check.CommentInterface
	}
	testCases := []testCase{
		{
			in:      "example.com/10.0.0.1",
			outZone: "example.com",
			outNs:   "10.0.0.1",
			comment: check.Commentf("Correct zone override."),
		},
		{
			in:      "example.com/10.0.0.1:10053",
			outZone: "example.com",
			outNs:   "10.0.0.1:10053",
			comment: check.Commentf("Correct zone override with custom ns port."),
		},
		{
			in:       "example.com:10.0.0.1",
			outError: true,
			comment:  check.Commentf("Incorrectly formatted zone override."),
		},
		{
			in:       "example.com/10.0.0.1:655350",
			outError: true,
			comment:  check.Commentf("Invalid nameserver port."),
		},
		{
			in:       "example.com/10.0.0.1:dns",
			outError: true,
			comment:  check.Commentf("Non-numeric nameserver port."),
		},
		{
			in:       "exa~mple.com/10.0.0.1",
			outError: true,
			comment:  check.Commentf("Invalid domain name."),
		},
		{
			in:       "example.com/foo.com",
			outError: true,
			comment:  check.Commentf("Nameserver is not an IP address."),
		},
	}
	for _, testCase := range testCases {
		zone, ns, err := ParseZoneOverride(testCase.in)
		if testCase.outError {
			c.Assert(err, check.NotNil, testCase.comment)
		} else {
			c.Assert(err, check.IsNil, testCase.comment)
		}
		c.Assert(zone, check.Equals, testCase.outZone, testCase.comment)
		c.Assert(ns, check.Equals, testCase.outNs, testCase.comment)
	}
}
