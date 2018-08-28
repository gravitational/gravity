package httplib

import (
	"net/http"
	"testing"

	. "gopkg.in/check.v1"
)

func TestHTTP(t *testing.T) { TestingT(t) }

type testHTTPSuite struct {
}

var _ = Suite(&testHTTPSuite{})

func (s *testHTTPSuite) TestSameOrigin(c *C) {

	type input struct {
		host      string
		referer   string
		origin    string
		proxyHost string
	}

	var host = "gravitational.com"

	var valid = []input{
		{host: host, referer: "https://gravitational.com"},
		{host: host, referer: "http://gravitational.com/test"},
		{host: host, origin: "https://gravitational.com"},
		{host: host, origin: "http://gravitational.com"},
		{host: host, origin: "https://proxy.com/test/test", proxyHost: "proxy.com"},
		{host: host, origin: "https://proxyA.com/test/test", proxyHost: "proxyA.com, proxyB.value"},
		{host: host, referer: "http://proxy.com", proxyHost: "proxy.com"},
	}

	var invalid = []input{
		{host: host},
		{host: host, referer: "gravitational.com"},
		{host: host, referer: "http://XXX.com/test"},
		{host: host, origin: "gravitational.com"},
		{host: host, origin: "http://XXX.com"},
		{host: host, origin: "https://proxy.com/test/test", proxyHost: "someotherproxy.com"},
		{host: host, origin: "https://proxy.com/test/test", proxyHost: "someotherproxy1.com, someotherproxy2.com"},
		{host: host, referer: "http://proxy.com", proxyHost: "someotherproxy.com"},
	}

	for _, i := range valid {
		r := s.makeRequest(i.host, i.referer, i.origin, i.proxyHost)
		err := VerifySameOrigin(&r)
		c.Assert(err, IsNil)
	}

	for _, i := range invalid {
		r := s.makeRequest(i.host, i.referer, i.origin, i.proxyHost)
		err := VerifySameOrigin(&r)
		c.Assert(err, NotNil)
	}
}

func (s *testHTTPSuite) makeRequest(host string, referer string, origin string, proxyHost string) http.Request {
	var header = make(http.Header)

	header["X-Forwarded-Host"] = []string{proxyHost}
	header["Referer"] = []string{referer}
	header["Origin"] = []string{origin}

	return http.Request{
		Header: header,
		Host:   host,
	}
}
