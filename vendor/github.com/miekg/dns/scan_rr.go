package dns

import (
	"encoding/base64"
	"net"
	"strconv"
	"strings"
)

// A remainder of the rdata with embedded spaces, return the parsed string (sans the spaces)
// or an error
func endingToString(c *zlexer, errstr string) (string, *ParseError) {
	var s string
	l, _ := c.Next() // zString
	for l.value != zNewline && l.value != zEOF {
		if l.err {
			return s, &ParseError{"", errstr, l}
		}
		switch l.value {
		case zString:
			s += l.token
		case zBlank: // Ok
		default:
			return "", &ParseError{"", errstr, l}
		}
		l, _ = c.Next()
	}

	return s, nil
}

// A remainder of the rdata with embedded spaces, split on unquoted whitespace
// and return the parsed string slice or an error
<<<<<<< HEAD
func endingToTxtSlice(c *zlexer, errstr string) ([]string, *ParseError) {
	// Get the remaining data until we see a zNewline
	l, _ := c.Next()
	if l.err {
		return nil, &ParseError{"", errstr, l}
=======
func endingToTxtSlice(c chan lex, errstr, f string) ([]string, *ParseError, string) {
	// Get the remaining data until we see a zNewline
	l := <-c
	if l.err {
		return nil, &ParseError{f, errstr, l}, ""
>>>>>>> 85acc1406... Bump K8s libraries to 1.13.4
	}

	// Build the slice
	s := make([]string, 0)
	quote := false
	empty := false
	for l.value != zNewline && l.value != zEOF {
		if l.err {
<<<<<<< HEAD
			return nil, &ParseError{"", errstr, l}
=======
			return nil, &ParseError{f, errstr, l}, ""
>>>>>>> 85acc1406... Bump K8s libraries to 1.13.4
		}
		switch l.value {
		case zString:
			empty = false
			if len(l.token) > 255 {
				// split up tokens that are larger than 255 into 255-chunks
				sx := []string{}
				p, i := 0, 255
				for {
					if i <= len(l.token) {
						sx = append(sx, l.token[p:i])
					} else {
						sx = append(sx, l.token[p:])
						break

					}
					p, i = p+255, i+255
				}
				s = append(s, sx...)
				break
			}

			s = append(s, l.token)
		case zBlank:
			if quote {
				// zBlank can only be seen in between txt parts.
<<<<<<< HEAD
				return nil, &ParseError{"", errstr, l}
=======
				return nil, &ParseError{f, errstr, l}, ""
>>>>>>> 85acc1406... Bump K8s libraries to 1.13.4
			}
		case zQuote:
			if empty && quote {
				s = append(s, "")
			}
			quote = !quote
			empty = true
		default:
<<<<<<< HEAD
			return nil, &ParseError{"", errstr, l}
		}
		l, _ = c.Next()
=======
			return nil, &ParseError{f, errstr, l}, ""
		}
		l = <-c
	}
	if quote {
		return nil, &ParseError{f, errstr, l}, ""
>>>>>>> 85acc1406... Bump K8s libraries to 1.13.4
	}

<<<<<<< HEAD
	if quote {
		return nil, &ParseError{"", errstr, l}
	}

	return s, nil
}

func (rr *A) parse(c *zlexer, o string) *ParseError {
	l, _ := c.Next()
=======
	l := <-c
	if l.length == 0 { // dynamic update rr.
		return rr, nil, ""
	}

>>>>>>> 85acc1406... Bump K8s libraries to 1.13.4
	rr.A = net.ParseIP(l.token)
	// IPv4 addresses cannot include ":".
	// We do this rather than use net.IP's To4() because
	// To4() treats IPv4-mapped IPv6 addresses as being
	// IPv4.
	isIPv4 := !strings.Contains(l.token, ":")
	if rr.A == nil || !isIPv4 || l.err {
		return &ParseError{"", "bad A A", l}
	}
	return slurpRemainder(c)
}

<<<<<<< HEAD
func (rr *AAAA) parse(c *zlexer, o string) *ParseError {
	l, _ := c.Next()
=======
func setAAAA(h RR_Header, c chan lex, o, f string) (RR, *ParseError, string) {
	rr := new(AAAA)
	rr.Hdr = h

	l := <-c
	if l.length == 0 { // dynamic update rr.
		return rr, nil, ""
	}

>>>>>>> 85acc1406... Bump K8s libraries to 1.13.4
	rr.AAAA = net.ParseIP(l.token)
	// IPv6 addresses must include ":", and IPv4
	// addresses cannot include ":".
	isIPv6 := strings.Contains(l.token, ":")
	if rr.AAAA == nil || !isIPv6 || l.err {
		return &ParseError{"", "bad AAAA AAAA", l}
	}
	return slurpRemainder(c)
}

<<<<<<< HEAD
func (rr *NS) parse(c *zlexer, o string) *ParseError {
	l, _ := c.Next()
	name, nameOk := toAbsoluteName(l.token, o)
	if l.err || !nameOk {
		return &ParseError{"", "bad NS Ns", l}
	}
	rr.Ns = name
	return slurpRemainder(c)
}

func (rr *PTR) parse(c *zlexer, o string) *ParseError {
	l, _ := c.Next()
	name, nameOk := toAbsoluteName(l.token, o)
	if l.err || !nameOk {
		return &ParseError{"", "bad PTR Ptr", l}
	}
	rr.Ptr = name
	return slurpRemainder(c)
}

func (rr *NSAPPTR) parse(c *zlexer, o string) *ParseError {
	l, _ := c.Next()
	name, nameOk := toAbsoluteName(l.token, o)
	if l.err || !nameOk {
		return &ParseError{"", "bad NSAP-PTR Ptr", l}
	}
	rr.Ptr = name
	return slurpRemainder(c)
}

func (rr *RP) parse(c *zlexer, o string) *ParseError {
	l, _ := c.Next()
	mbox, mboxOk := toAbsoluteName(l.token, o)
	if l.err || !mboxOk {
		return &ParseError{"", "bad RP Mbox", l}
	}
	rr.Mbox = mbox

	c.Next() // zBlank
	l, _ = c.Next()
=======
func setNS(h RR_Header, c chan lex, o, f string) (RR, *ParseError, string) {
	rr := new(NS)
	rr.Hdr = h

	l := <-c
	rr.Ns = l.token
	if l.length == 0 { // dynamic update rr.
		return rr, nil, ""
	}

	name, nameOk := toAbsoluteName(l.token, o)
	if l.err || !nameOk {
		return nil, &ParseError{f, "bad NS Ns", l}, ""
	}
	rr.Ns = name
	return rr, nil, ""
}

func setPTR(h RR_Header, c chan lex, o, f string) (RR, *ParseError, string) {
	rr := new(PTR)
	rr.Hdr = h

	l := <-c
	rr.Ptr = l.token
	if l.length == 0 { // dynamic update rr.
		return rr, nil, ""
	}

	name, nameOk := toAbsoluteName(l.token, o)
	if l.err || !nameOk {
		return nil, &ParseError{f, "bad PTR Ptr", l}, ""
	}
	rr.Ptr = name
	return rr, nil, ""
}

func setNSAPPTR(h RR_Header, c chan lex, o, f string) (RR, *ParseError, string) {
	rr := new(NSAPPTR)
	rr.Hdr = h

	l := <-c
	rr.Ptr = l.token
	if l.length == 0 { // dynamic update rr.
		return rr, nil, ""
	}

	name, nameOk := toAbsoluteName(l.token, o)
	if l.err || !nameOk {
		return nil, &ParseError{f, "bad NSAP-PTR Ptr", l}, ""
	}
	rr.Ptr = name
	return rr, nil, ""
}

func setRP(h RR_Header, c chan lex, o, f string) (RR, *ParseError, string) {
	rr := new(RP)
	rr.Hdr = h

	l := <-c
	rr.Mbox = l.token
	if l.length == 0 { // dynamic update rr.
		return rr, nil, ""
	}

	mbox, mboxOk := toAbsoluteName(l.token, o)
	if l.err || !mboxOk {
		return nil, &ParseError{f, "bad RP Mbox", l}, ""
	}
	rr.Mbox = mbox

	<-c // zBlank
	l = <-c
>>>>>>> 85acc1406... Bump K8s libraries to 1.13.4
	rr.Txt = l.token

	txt, txtOk := toAbsoluteName(l.token, o)
	if l.err || !txtOk {
<<<<<<< HEAD
		return &ParseError{"", "bad RP Txt", l}
	}
	rr.Txt = txt
=======
		return nil, &ParseError{f, "bad RP Txt", l}, ""
	}
	rr.Txt = txt

	return rr, nil, ""
}
>>>>>>> 85acc1406... Bump K8s libraries to 1.13.4

	return slurpRemainder(c)
}

<<<<<<< HEAD
func (rr *MR) parse(c *zlexer, o string) *ParseError {
	l, _ := c.Next()
	name, nameOk := toAbsoluteName(l.token, o)
	if l.err || !nameOk {
		return &ParseError{"", "bad MR Mr", l}
	}
	rr.Mr = name
	return slurpRemainder(c)
}

func (rr *MB) parse(c *zlexer, o string) *ParseError {
	l, _ := c.Next()
	name, nameOk := toAbsoluteName(l.token, o)
	if l.err || !nameOk {
		return &ParseError{"", "bad MB Mb", l}
	}
	rr.Mb = name
	return slurpRemainder(c)
}

func (rr *MG) parse(c *zlexer, o string) *ParseError {
	l, _ := c.Next()
	name, nameOk := toAbsoluteName(l.token, o)
	if l.err || !nameOk {
		return &ParseError{"", "bad MG Mg", l}
	}
	rr.Mg = name
	return slurpRemainder(c)
=======
	l := <-c
	rr.Mr = l.token
	if l.length == 0 { // dynamic update rr.
		return rr, nil, ""
	}

	name, nameOk := toAbsoluteName(l.token, o)
	if l.err || !nameOk {
		return nil, &ParseError{f, "bad MR Mr", l}, ""
	}
	rr.Mr = name
	return rr, nil, ""
}

func setMB(h RR_Header, c chan lex, o, f string) (RR, *ParseError, string) {
	rr := new(MB)
	rr.Hdr = h

	l := <-c
	rr.Mb = l.token
	if l.length == 0 { // dynamic update rr.
		return rr, nil, ""
	}

	name, nameOk := toAbsoluteName(l.token, o)
	if l.err || !nameOk {
		return nil, &ParseError{f, "bad MB Mb", l}, ""
	}
	rr.Mb = name
	return rr, nil, ""
}

func setMG(h RR_Header, c chan lex, o, f string) (RR, *ParseError, string) {
	rr := new(MG)
	rr.Hdr = h

	l := <-c
	rr.Mg = l.token
	if l.length == 0 { // dynamic update rr.
		return rr, nil, ""
	}

	name, nameOk := toAbsoluteName(l.token, o)
	if l.err || !nameOk {
		return nil, &ParseError{f, "bad MG Mg", l}, ""
	}
	rr.Mg = name
	return rr, nil, ""
>>>>>>> 85acc1406... Bump K8s libraries to 1.13.4
}

func (rr *HINFO) parse(c *zlexer, o string) *ParseError {
	chunks, e := endingToTxtSlice(c, "bad HINFO Fields")
	if e != nil {
		return e
	}

	if ln := len(chunks); ln == 0 {
		return nil
	} else if ln == 1 {
		// Can we split it?
		if out := strings.Fields(chunks[0]); len(out) > 1 {
			chunks = out
		} else {
			chunks = append(chunks, "")
		}
	}

	rr.Cpu = chunks[0]
	rr.Os = strings.Join(chunks[1:], " ")

	return nil
}

<<<<<<< HEAD
func (rr *MINFO) parse(c *zlexer, o string) *ParseError {
	l, _ := c.Next()
	rmail, rmailOk := toAbsoluteName(l.token, o)
	if l.err || !rmailOk {
		return &ParseError{"", "bad MINFO Rmail", l}
	}
	rr.Rmail = rmail

	c.Next() // zBlank
	l, _ = c.Next()
=======
func setMINFO(h RR_Header, c chan lex, o, f string) (RR, *ParseError, string) {
	rr := new(MINFO)
	rr.Hdr = h

	l := <-c
	rr.Rmail = l.token
	if l.length == 0 { // dynamic update rr.
		return rr, nil, ""
	}

	rmail, rmailOk := toAbsoluteName(l.token, o)
	if l.err || !rmailOk {
		return nil, &ParseError{f, "bad MINFO Rmail", l}, ""
	}
	rr.Rmail = rmail

	<-c // zBlank
	l = <-c
>>>>>>> 85acc1406... Bump K8s libraries to 1.13.4
	rr.Email = l.token

	email, emailOk := toAbsoluteName(l.token, o)
	if l.err || !emailOk {
<<<<<<< HEAD
		return &ParseError{"", "bad MINFO Email", l}
	}
	rr.Email = email
=======
		return nil, &ParseError{f, "bad MINFO Email", l}, ""
	}
	rr.Email = email

	return rr, nil, ""
}
>>>>>>> 85acc1406... Bump K8s libraries to 1.13.4

	return slurpRemainder(c)
}

<<<<<<< HEAD
func (rr *MF) parse(c *zlexer, o string) *ParseError {
	l, _ := c.Next()
	name, nameOk := toAbsoluteName(l.token, o)
	if l.err || !nameOk {
		return &ParseError{"", "bad MF Mf", l}
	}
	rr.Mf = name
	return slurpRemainder(c)
}

func (rr *MD) parse(c *zlexer, o string) *ParseError {
	l, _ := c.Next()
	name, nameOk := toAbsoluteName(l.token, o)
	if l.err || !nameOk {
		return &ParseError{"", "bad MD Md", l}
	}
	rr.Md = name
	return slurpRemainder(c)
}

func (rr *MX) parse(c *zlexer, o string) *ParseError {
	l, _ := c.Next()
=======
	l := <-c
	rr.Mf = l.token
	if l.length == 0 { // dynamic update rr.
		return rr, nil, ""
	}

	name, nameOk := toAbsoluteName(l.token, o)
	if l.err || !nameOk {
		return nil, &ParseError{f, "bad MF Mf", l}, ""
	}
	rr.Mf = name
	return rr, nil, ""
}

func setMD(h RR_Header, c chan lex, o, f string) (RR, *ParseError, string) {
	rr := new(MD)
	rr.Hdr = h

	l := <-c
	rr.Md = l.token
	if l.length == 0 { // dynamic update rr.
		return rr, nil, ""
	}

	name, nameOk := toAbsoluteName(l.token, o)
	if l.err || !nameOk {
		return nil, &ParseError{f, "bad MD Md", l}, ""
	}
	rr.Md = name
	return rr, nil, ""
}

func setMX(h RR_Header, c chan lex, o, f string) (RR, *ParseError, string) {
	rr := new(MX)
	rr.Hdr = h

	l := <-c
	if l.length == 0 { // dynamic update rr.
		return rr, nil, ""
	}

>>>>>>> 85acc1406... Bump K8s libraries to 1.13.4
	i, e := strconv.ParseUint(l.token, 10, 16)
	if e != nil || l.err {
		return &ParseError{"", "bad MX Pref", l}
	}
	rr.Preference = uint16(i)

<<<<<<< HEAD
	c.Next()        // zBlank
	l, _ = c.Next() // zString
=======
	<-c     // zBlank
	l = <-c // zString
>>>>>>> 85acc1406... Bump K8s libraries to 1.13.4
	rr.Mx = l.token

	name, nameOk := toAbsoluteName(l.token, o)
	if l.err || !nameOk {
<<<<<<< HEAD
		return &ParseError{"", "bad MX Mx", l}
	}
	rr.Mx = name

	return slurpRemainder(c)
}

func (rr *RT) parse(c *zlexer, o string) *ParseError {
	l, _ := c.Next()
=======
		return nil, &ParseError{f, "bad MX Mx", l}, ""
	}
	rr.Mx = name

	return rr, nil, ""
}

func setRT(h RR_Header, c chan lex, o, f string) (RR, *ParseError, string) {
	rr := new(RT)
	rr.Hdr = h

	l := <-c
	if l.length == 0 { // dynamic update rr.
		return rr, nil, ""
	}

>>>>>>> 85acc1406... Bump K8s libraries to 1.13.4
	i, e := strconv.ParseUint(l.token, 10, 16)
	if e != nil {
		return &ParseError{"", "bad RT Preference", l}
	}
	rr.Preference = uint16(i)

<<<<<<< HEAD
	c.Next()        // zBlank
	l, _ = c.Next() // zString
=======
	<-c     // zBlank
	l = <-c // zString
>>>>>>> 85acc1406... Bump K8s libraries to 1.13.4
	rr.Host = l.token

	name, nameOk := toAbsoluteName(l.token, o)
	if l.err || !nameOk {
<<<<<<< HEAD
		return &ParseError{"", "bad RT Host", l}
	}
	rr.Host = name
=======
		return nil, &ParseError{f, "bad RT Host", l}, ""
	}
	rr.Host = name

	return rr, nil, ""
}
>>>>>>> 85acc1406... Bump K8s libraries to 1.13.4

	return slurpRemainder(c)
}

<<<<<<< HEAD
func (rr *AFSDB) parse(c *zlexer, o string) *ParseError {
	l, _ := c.Next()
=======
	l := <-c
	if l.length == 0 { // dynamic update rr.
		return rr, nil, ""
	}

>>>>>>> 85acc1406... Bump K8s libraries to 1.13.4
	i, e := strconv.ParseUint(l.token, 10, 16)
	if e != nil || l.err {
		return &ParseError{"", "bad AFSDB Subtype", l}
	}
	rr.Subtype = uint16(i)

<<<<<<< HEAD
	c.Next()        // zBlank
	l, _ = c.Next() // zString
=======
	<-c     // zBlank
	l = <-c // zString
>>>>>>> 85acc1406... Bump K8s libraries to 1.13.4
	rr.Hostname = l.token

	name, nameOk := toAbsoluteName(l.token, o)
	if l.err || !nameOk {
<<<<<<< HEAD
		return &ParseError{"", "bad AFSDB Hostname", l}
	}
	rr.Hostname = name
	return slurpRemainder(c)
}

func (rr *X25) parse(c *zlexer, o string) *ParseError {
	l, _ := c.Next()
=======
		return nil, &ParseError{f, "bad AFSDB Hostname", l}, ""
	}
	rr.Hostname = name
	return rr, nil, ""
}

func setX25(h RR_Header, c chan lex, o, f string) (RR, *ParseError, string) {
	rr := new(X25)
	rr.Hdr = h

	l := <-c
	if l.length == 0 { // dynamic update rr.
		return rr, nil, ""
	}

>>>>>>> 85acc1406... Bump K8s libraries to 1.13.4
	if l.err {
		return &ParseError{"", "bad X25 PSDNAddress", l}
	}
	rr.PSDNAddress = l.token
	return slurpRemainder(c)
}

<<<<<<< HEAD
func (rr *KX) parse(c *zlexer, o string) *ParseError {
	l, _ := c.Next()
=======
func setKX(h RR_Header, c chan lex, o, f string) (RR, *ParseError, string) {
	rr := new(KX)
	rr.Hdr = h

	l := <-c
	if l.length == 0 { // dynamic update rr.
		return rr, nil, ""
	}

>>>>>>> 85acc1406... Bump K8s libraries to 1.13.4
	i, e := strconv.ParseUint(l.token, 10, 16)
	if e != nil || l.err {
		return &ParseError{"", "bad KX Pref", l}
	}
	rr.Preference = uint16(i)

<<<<<<< HEAD
	c.Next()        // zBlank
	l, _ = c.Next() // zString
=======
	<-c     // zBlank
	l = <-c // zString
>>>>>>> 85acc1406... Bump K8s libraries to 1.13.4
	rr.Exchanger = l.token

	name, nameOk := toAbsoluteName(l.token, o)
	if l.err || !nameOk {
<<<<<<< HEAD
		return &ParseError{"", "bad KX Exchanger", l}
	}
	rr.Exchanger = name
	return slurpRemainder(c)
}

func (rr *CNAME) parse(c *zlexer, o string) *ParseError {
	l, _ := c.Next()
	name, nameOk := toAbsoluteName(l.token, o)
	if l.err || !nameOk {
		return &ParseError{"", "bad CNAME Target", l}
	}
	rr.Target = name
	return slurpRemainder(c)
}

func (rr *DNAME) parse(c *zlexer, o string) *ParseError {
	l, _ := c.Next()
	name, nameOk := toAbsoluteName(l.token, o)
	if l.err || !nameOk {
		return &ParseError{"", "bad DNAME Target", l}
	}
	rr.Target = name
	return slurpRemainder(c)
}

func (rr *SOA) parse(c *zlexer, o string) *ParseError {
	l, _ := c.Next()
	ns, nsOk := toAbsoluteName(l.token, o)
	if l.err || !nsOk {
		return &ParseError{"", "bad SOA Ns", l}
	}
	rr.Ns = ns

	c.Next() // zBlank
	l, _ = c.Next()
=======
		return nil, &ParseError{f, "bad KX Exchanger", l}, ""
	}
	rr.Exchanger = name
	return rr, nil, ""
}

func setCNAME(h RR_Header, c chan lex, o, f string) (RR, *ParseError, string) {
	rr := new(CNAME)
	rr.Hdr = h

	l := <-c
	rr.Target = l.token
	if l.length == 0 { // dynamic update rr.
		return rr, nil, ""
	}

	name, nameOk := toAbsoluteName(l.token, o)
	if l.err || !nameOk {
		return nil, &ParseError{f, "bad CNAME Target", l}, ""
	}
	rr.Target = name
	return rr, nil, ""
}

func setDNAME(h RR_Header, c chan lex, o, f string) (RR, *ParseError, string) {
	rr := new(DNAME)
	rr.Hdr = h

	l := <-c
	rr.Target = l.token
	if l.length == 0 { // dynamic update rr.
		return rr, nil, ""
	}

	name, nameOk := toAbsoluteName(l.token, o)
	if l.err || !nameOk {
		return nil, &ParseError{f, "bad DNAME Target", l}, ""
	}
	rr.Target = name
	return rr, nil, ""
}

func setSOA(h RR_Header, c chan lex, o, f string) (RR, *ParseError, string) {
	rr := new(SOA)
	rr.Hdr = h

	l := <-c
	rr.Ns = l.token
	if l.length == 0 { // dynamic update rr.
		return rr, nil, ""
	}

	ns, nsOk := toAbsoluteName(l.token, o)
	if l.err || !nsOk {
		return nil, &ParseError{f, "bad SOA Ns", l}, ""
	}
	rr.Ns = ns

	<-c // zBlank
	l = <-c
>>>>>>> 85acc1406... Bump K8s libraries to 1.13.4
	rr.Mbox = l.token

	mbox, mboxOk := toAbsoluteName(l.token, o)
	if l.err || !mboxOk {
<<<<<<< HEAD
		return &ParseError{"", "bad SOA Mbox", l}
	}
	rr.Mbox = mbox

	c.Next() // zBlank
=======
		return nil, &ParseError{f, "bad SOA Mbox", l}, ""
	}
	rr.Mbox = mbox

	<-c // zBlank
>>>>>>> 85acc1406... Bump K8s libraries to 1.13.4

	var (
		v  uint32
		ok bool
	)
	for i := 0; i < 5; i++ {
		l, _ = c.Next()
		if l.err {
			return &ParseError{"", "bad SOA zone parameter", l}
		}
		if j, e := strconv.ParseUint(l.token, 10, 32); e != nil {
			if i == 0 {
				// Serial must be a number
<<<<<<< HEAD
				return &ParseError{"", "bad SOA zone parameter", l}
			}
			// We allow other fields to be unitful duration strings
			if v, ok = stringToTTL(l.token); !ok {
				return &ParseError{"", "bad SOA zone parameter", l}
=======
				return nil, &ParseError{f, "bad SOA zone parameter", l}, ""
			}
			// We allow other fields to be unitful duration strings
			if v, ok = stringToTTL(l.token); !ok {
				return nil, &ParseError{f, "bad SOA zone parameter", l}, ""
>>>>>>> 85acc1406... Bump K8s libraries to 1.13.4

			}
		} else {
			v = uint32(j)
		}
		switch i {
		case 0:
			rr.Serial = v
			c.Next() // zBlank
		case 1:
			rr.Refresh = v
			c.Next() // zBlank
		case 2:
			rr.Retry = v
			c.Next() // zBlank
		case 3:
			rr.Expire = v
			c.Next() // zBlank
		case 4:
			rr.Minttl = v
		}
	}
	return slurpRemainder(c)
}

<<<<<<< HEAD
func (rr *SRV) parse(c *zlexer, o string) *ParseError {
	l, _ := c.Next()
=======
func setSRV(h RR_Header, c chan lex, o, f string) (RR, *ParseError, string) {
	rr := new(SRV)
	rr.Hdr = h

	l := <-c
	if l.length == 0 { // dynamic update rr.
		return rr, nil, ""
	}

>>>>>>> 85acc1406... Bump K8s libraries to 1.13.4
	i, e := strconv.ParseUint(l.token, 10, 16)
	if e != nil || l.err {
		return &ParseError{"", "bad SRV Priority", l}
	}
	rr.Priority = uint16(i)

<<<<<<< HEAD
	c.Next()        // zBlank
	l, _ = c.Next() // zString
=======
	<-c     // zBlank
	l = <-c // zString
>>>>>>> 85acc1406... Bump K8s libraries to 1.13.4
	i, e = strconv.ParseUint(l.token, 10, 16)
	if e != nil || l.err {
		return &ParseError{"", "bad SRV Weight", l}
	}
	rr.Weight = uint16(i)

<<<<<<< HEAD
	c.Next()        // zBlank
	l, _ = c.Next() // zString
=======
	<-c     // zBlank
	l = <-c // zString
>>>>>>> 85acc1406... Bump K8s libraries to 1.13.4
	i, e = strconv.ParseUint(l.token, 10, 16)
	if e != nil || l.err {
		return &ParseError{"", "bad SRV Port", l}
	}
	rr.Port = uint16(i)

<<<<<<< HEAD
	c.Next()        // zBlank
	l, _ = c.Next() // zString
=======
	<-c     // zBlank
	l = <-c // zString
>>>>>>> 85acc1406... Bump K8s libraries to 1.13.4
	rr.Target = l.token

	name, nameOk := toAbsoluteName(l.token, o)
	if l.err || !nameOk {
<<<<<<< HEAD
		return &ParseError{"", "bad SRV Target", l}
	}
	rr.Target = name
	return slurpRemainder(c)
}

func (rr *NAPTR) parse(c *zlexer, o string) *ParseError {
	l, _ := c.Next()
=======
		return nil, &ParseError{f, "bad SRV Target", l}, ""
	}
	rr.Target = name
	return rr, nil, ""
}

func setNAPTR(h RR_Header, c chan lex, o, f string) (RR, *ParseError, string) {
	rr := new(NAPTR)
	rr.Hdr = h

	l := <-c
	if l.length == 0 { // dynamic update rr.
		return rr, nil, ""
	}

>>>>>>> 85acc1406... Bump K8s libraries to 1.13.4
	i, e := strconv.ParseUint(l.token, 10, 16)
	if e != nil || l.err {
		return &ParseError{"", "bad NAPTR Order", l}
	}
	rr.Order = uint16(i)

<<<<<<< HEAD
	c.Next()        // zBlank
	l, _ = c.Next() // zString
=======
	<-c     // zBlank
	l = <-c // zString
>>>>>>> 85acc1406... Bump K8s libraries to 1.13.4
	i, e = strconv.ParseUint(l.token, 10, 16)
	if e != nil || l.err {
		return &ParseError{"", "bad NAPTR Preference", l}
	}
	rr.Preference = uint16(i)

	// Flags
	c.Next()        // zBlank
	l, _ = c.Next() // _QUOTE
	if l.value != zQuote {
		return &ParseError{"", "bad NAPTR Flags", l}
	}
	l, _ = c.Next() // Either String or Quote
	if l.value == zString {
		rr.Flags = l.token
		l, _ = c.Next() // _QUOTE
		if l.value != zQuote {
			return &ParseError{"", "bad NAPTR Flags", l}
		}
	} else if l.value == zQuote {
		rr.Flags = ""
	} else {
		return &ParseError{"", "bad NAPTR Flags", l}
	}

	// Service
	c.Next()        // zBlank
	l, _ = c.Next() // _QUOTE
	if l.value != zQuote {
		return &ParseError{"", "bad NAPTR Service", l}
	}
	l, _ = c.Next() // Either String or Quote
	if l.value == zString {
		rr.Service = l.token
		l, _ = c.Next() // _QUOTE
		if l.value != zQuote {
			return &ParseError{"", "bad NAPTR Service", l}
		}
	} else if l.value == zQuote {
		rr.Service = ""
	} else {
		return &ParseError{"", "bad NAPTR Service", l}
	}

	// Regexp
	c.Next()        // zBlank
	l, _ = c.Next() // _QUOTE
	if l.value != zQuote {
		return &ParseError{"", "bad NAPTR Regexp", l}
	}
	l, _ = c.Next() // Either String or Quote
	if l.value == zString {
		rr.Regexp = l.token
		l, _ = c.Next() // _QUOTE
		if l.value != zQuote {
			return &ParseError{"", "bad NAPTR Regexp", l}
		}
	} else if l.value == zQuote {
		rr.Regexp = ""
	} else {
		return &ParseError{"", "bad NAPTR Regexp", l}
	}

	// After quote no space??
	c.Next()        // zBlank
	l, _ = c.Next() // zString
	rr.Replacement = l.token

	name, nameOk := toAbsoluteName(l.token, o)
	if l.err || !nameOk {
<<<<<<< HEAD
		return &ParseError{"", "bad NAPTR Replacement", l}
	}
	rr.Replacement = name
	return slurpRemainder(c)
}

func (rr *TALINK) parse(c *zlexer, o string) *ParseError {
	l, _ := c.Next()
	previousName, previousNameOk := toAbsoluteName(l.token, o)
	if l.err || !previousNameOk {
		return &ParseError{"", "bad TALINK PreviousName", l}
	}
	rr.PreviousName = previousName

	c.Next() // zBlank
	l, _ = c.Next()
=======
		return nil, &ParseError{f, "bad NAPTR Replacement", l}, ""
	}
	rr.Replacement = name
	return rr, nil, ""
}

func setTALINK(h RR_Header, c chan lex, o, f string) (RR, *ParseError, string) {
	rr := new(TALINK)
	rr.Hdr = h

	l := <-c
	rr.PreviousName = l.token
	if l.length == 0 { // dynamic update rr.
		return rr, nil, ""
	}

	previousName, previousNameOk := toAbsoluteName(l.token, o)
	if l.err || !previousNameOk {
		return nil, &ParseError{f, "bad TALINK PreviousName", l}, ""
	}
	rr.PreviousName = previousName

	<-c // zBlank
	l = <-c
>>>>>>> 85acc1406... Bump K8s libraries to 1.13.4
	rr.NextName = l.token

	nextName, nextNameOk := toAbsoluteName(l.token, o)
	if l.err || !nextNameOk {
<<<<<<< HEAD
		return &ParseError{"", "bad TALINK NextName", l}
	}
	rr.NextName = nextName

	return slurpRemainder(c)
=======
		return nil, &ParseError{f, "bad TALINK NextName", l}, ""
	}
	rr.NextName = nextName

	return rr, nil, ""
>>>>>>> 85acc1406... Bump K8s libraries to 1.13.4
}

func (rr *LOC) parse(c *zlexer, o string) *ParseError {
	// Non zero defaults for LOC record, see RFC 1876, Section 3.
	rr.HorizPre = 165 // 10000
	rr.VertPre = 162  // 10
	rr.Size = 18      // 1
	ok := false

	// North
<<<<<<< HEAD
	l, _ := c.Next()
=======
	l := <-c
	if l.length == 0 { // dynamic update rr.
		return rr, nil, ""
	}
>>>>>>> 85acc1406... Bump K8s libraries to 1.13.4
	i, e := strconv.ParseUint(l.token, 10, 32)
	if e != nil || l.err {
		return &ParseError{"", "bad LOC Latitude", l}
	}
	rr.Latitude = 1000 * 60 * 60 * uint32(i)

	c.Next() // zBlank
	// Either number, 'N' or 'S'
	l, _ = c.Next()
	if rr.Latitude, ok = locCheckNorth(l.token, rr.Latitude); ok {
		goto East
	}
	i, e = strconv.ParseUint(l.token, 10, 32)
	if e != nil || l.err {
		return &ParseError{"", "bad LOC Latitude minutes", l}
	}
	rr.Latitude += 1000 * 60 * uint32(i)

	c.Next() // zBlank
	l, _ = c.Next()
	if i, e := strconv.ParseFloat(l.token, 32); e != nil || l.err {
		return &ParseError{"", "bad LOC Latitude seconds", l}
	} else {
		rr.Latitude += uint32(1000 * i)
	}
	c.Next() // zBlank
	// Either number, 'N' or 'S'
	l, _ = c.Next()
	if rr.Latitude, ok = locCheckNorth(l.token, rr.Latitude); ok {
		goto East
	}
	// If still alive, flag an error
	return &ParseError{"", "bad LOC Latitude North/South", l}

East:
	// East
<<<<<<< HEAD
	c.Next() // zBlank
	l, _ = c.Next()
	if i, e := strconv.ParseUint(l.token, 10, 32); e != nil || l.err {
		return &ParseError{"", "bad LOC Longitude", l}
=======
	<-c // zBlank
	l = <-c
	if i, e := strconv.ParseUint(l.token, 10, 32); e != nil || l.err {
		return nil, &ParseError{f, "bad LOC Longitude", l}, ""
>>>>>>> 85acc1406... Bump K8s libraries to 1.13.4
	} else {
		rr.Longitude = 1000 * 60 * 60 * uint32(i)
	}
	c.Next() // zBlank
	// Either number, 'E' or 'W'
	l, _ = c.Next()
	if rr.Longitude, ok = locCheckEast(l.token, rr.Longitude); ok {
		goto Altitude
	}
	if i, e := strconv.ParseUint(l.token, 10, 32); e != nil || l.err {
<<<<<<< HEAD
		return &ParseError{"", "bad LOC Longitude minutes", l}
=======
		return nil, &ParseError{f, "bad LOC Longitude minutes", l}, ""
>>>>>>> 85acc1406... Bump K8s libraries to 1.13.4
	} else {
		rr.Longitude += 1000 * 60 * uint32(i)
	}
	c.Next() // zBlank
	l, _ = c.Next()
	if i, e := strconv.ParseFloat(l.token, 32); e != nil || l.err {
		return &ParseError{"", "bad LOC Longitude seconds", l}
	} else {
		rr.Longitude += uint32(1000 * i)
	}
	c.Next() // zBlank
	// Either number, 'E' or 'W'
	l, _ = c.Next()
	if rr.Longitude, ok = locCheckEast(l.token, rr.Longitude); ok {
		goto Altitude
	}
	// If still alive, flag an error
	return &ParseError{"", "bad LOC Longitude East/West", l}

Altitude:
	c.Next() // zBlank
	l, _ = c.Next()
	if len(l.token) == 0 || l.err {
		return &ParseError{"", "bad LOC Altitude", l}
	}
	if l.token[len(l.token)-1] == 'M' || l.token[len(l.token)-1] == 'm' {
		l.token = l.token[0 : len(l.token)-1]
	}
	if i, e := strconv.ParseFloat(l.token, 32); e != nil {
		return &ParseError{"", "bad LOC Altitude", l}
	} else {
		rr.Altitude = uint32(i*100.0 + 10000000.0 + 0.5)
	}

	// And now optionally the other values
	l, _ = c.Next()
	count := 0
	for l.value != zNewline && l.value != zEOF {
		switch l.value {
		case zString:
			switch count {
			case 0: // Size
				e, m, ok := stringToCm(l.token)
				if !ok {
					return &ParseError{"", "bad LOC Size", l}
				}
				rr.Size = e&0x0f | m<<4&0xf0
			case 1: // HorizPre
				e, m, ok := stringToCm(l.token)
				if !ok {
					return &ParseError{"", "bad LOC HorizPre", l}
				}
				rr.HorizPre = e&0x0f | m<<4&0xf0
			case 2: // VertPre
				e, m, ok := stringToCm(l.token)
				if !ok {
					return &ParseError{"", "bad LOC VertPre", l}
				}
				rr.VertPre = e&0x0f | m<<4&0xf0
			}
			count++
		case zBlank:
			// Ok
		default:
			return &ParseError{"", "bad LOC Size, HorizPre or VertPre", l}
		}
		l, _ = c.Next()
	}
	return nil
}

func (rr *HIP) parse(c *zlexer, o string) *ParseError {
	// HitLength is not represented
<<<<<<< HEAD
	l, _ := c.Next()
=======
	l := <-c
	if l.length == 0 { // dynamic update rr.
		return rr, nil, l.comment
	}

>>>>>>> 85acc1406... Bump K8s libraries to 1.13.4
	i, e := strconv.ParseUint(l.token, 10, 8)
	if e != nil || l.err {
		return &ParseError{"", "bad HIP PublicKeyAlgorithm", l}
	}
	rr.PublicKeyAlgorithm = uint8(i)

<<<<<<< HEAD
	c.Next()        // zBlank
	l, _ = c.Next() // zString
	if len(l.token) == 0 || l.err {
		return &ParseError{"", "bad HIP Hit", l}
=======
	<-c     // zBlank
	l = <-c // zString
	if l.length == 0 || l.err {
		return nil, &ParseError{f, "bad HIP Hit", l}, ""
>>>>>>> 85acc1406... Bump K8s libraries to 1.13.4
	}
	rr.Hit = l.token // This can not contain spaces, see RFC 5205 Section 6.
	rr.HitLength = uint8(len(rr.Hit)) / 2

	c.Next()        // zBlank
	l, _ = c.Next() // zString
	if len(l.token) == 0 || l.err {
		return &ParseError{"", "bad HIP PublicKey", l}
	}
	rr.PublicKey = l.token // This cannot contain spaces
	rr.PublicKeyLength = uint16(base64.StdEncoding.DecodedLen(len(rr.PublicKey)))

	// RendezvousServers (if any)
	l, _ = c.Next()
	var xs []string
	for l.value != zNewline && l.value != zEOF {
		switch l.value {
		case zString:
			name, nameOk := toAbsoluteName(l.token, o)
			if l.err || !nameOk {
<<<<<<< HEAD
				return &ParseError{"", "bad HIP RendezvousServers", l}
=======
				return nil, &ParseError{f, "bad HIP RendezvousServers", l}, ""
>>>>>>> 85acc1406... Bump K8s libraries to 1.13.4
			}
			xs = append(xs, name)
		case zBlank:
			// Ok
		default:
			return &ParseError{"", "bad HIP RendezvousServers", l}
		}
		l, _ = c.Next()
	}

	rr.RendezvousServers = xs
	return nil
}

<<<<<<< HEAD
func (rr *CERT) parse(c *zlexer, o string) *ParseError {
	l, _ := c.Next()
	if v, ok := StringToCertType[l.token]; ok {
		rr.Type = v
	} else if i, e := strconv.ParseUint(l.token, 10, 16); e != nil {
		return &ParseError{"", "bad CERT Type", l}
	} else {
		rr.Type = uint16(i)
	}
	c.Next()        // zBlank
	l, _ = c.Next() // zString
=======
func setCERT(h RR_Header, c chan lex, o, f string) (RR, *ParseError, string) {
	rr := new(CERT)
	rr.Hdr = h

	l := <-c
	if l.length == 0 { // dynamic update rr.
		return rr, nil, l.comment
	}

	if v, ok := StringToCertType[l.token]; ok {
		rr.Type = v
	} else if i, e := strconv.ParseUint(l.token, 10, 16); e != nil {
		return nil, &ParseError{f, "bad CERT Type", l}, ""
	} else {
		rr.Type = uint16(i)
	}
	<-c     // zBlank
	l = <-c // zString
>>>>>>> 85acc1406... Bump K8s libraries to 1.13.4
	i, e := strconv.ParseUint(l.token, 10, 16)
	if e != nil || l.err {
		return &ParseError{"", "bad CERT KeyTag", l}
	}
	rr.KeyTag = uint16(i)
	c.Next()        // zBlank
	l, _ = c.Next() // zString
	if v, ok := StringToAlgorithm[l.token]; ok {
		rr.Algorithm = v
	} else if i, e := strconv.ParseUint(l.token, 10, 8); e != nil {
<<<<<<< HEAD
		return &ParseError{"", "bad CERT Algorithm", l}
=======
		return nil, &ParseError{f, "bad CERT Algorithm", l}, ""
>>>>>>> 85acc1406... Bump K8s libraries to 1.13.4
	} else {
		rr.Algorithm = uint8(i)
	}
	s, e1 := endingToString(c, "bad CERT Certificate")
	if e1 != nil {
		return e1
	}
	rr.Certificate = s
	return nil
}

func (rr *OPENPGPKEY) parse(c *zlexer, o string) *ParseError {
	s, e := endingToString(c, "bad OPENPGPKEY PublicKey")
	if e != nil {
		return e
	}
	rr.PublicKey = s
	return nil
}

<<<<<<< HEAD
func (rr *CSYNC) parse(c *zlexer, o string) *ParseError {
	l, _ := c.Next()
	j, e := strconv.ParseUint(l.token, 10, 32)
	if e != nil {
		// Serial must be a number
		return &ParseError{"", "bad CSYNC serial", l}
=======
func setCSYNC(h RR_Header, c chan lex, o, f string) (RR, *ParseError, string) {
	rr := new(CSYNC)
	rr.Hdr = h

	l := <-c
	if l.length == 0 { // dynamic update rr.
		return rr, nil, l.comment
	}
	j, e := strconv.ParseUint(l.token, 10, 32)
	if e != nil {
		// Serial must be a number
		return nil, &ParseError{f, "bad CSYNC serial", l}, ""
	}
	rr.Serial = uint32(j)

	<-c // zBlank

	l = <-c
	j, e = strconv.ParseUint(l.token, 10, 16)
	if e != nil {
		// Serial must be a number
		return nil, &ParseError{f, "bad CSYNC flags", l}, ""
	}
	rr.Flags = uint16(j)

	rr.TypeBitMap = make([]uint16, 0)
	var (
		k  uint16
		ok bool
	)
	l = <-c
	for l.value != zNewline && l.value != zEOF {
		switch l.value {
		case zBlank:
			// Ok
		case zString:
			if k, ok = StringToType[l.tokenUpper]; !ok {
				if k, ok = typeToInt(l.tokenUpper); !ok {
					return nil, &ParseError{f, "bad CSYNC TypeBitMap", l}, ""
				}
			}
			rr.TypeBitMap = append(rr.TypeBitMap, k)
		default:
			return nil, &ParseError{f, "bad CSYNC TypeBitMap", l}, ""
		}
		l = <-c
	}
	return rr, nil, l.comment
}

func setSIG(h RR_Header, c chan lex, o, f string) (RR, *ParseError, string) {
	r, e, s := setRRSIG(h, c, o, f)
	if r != nil {
		return &SIG{*r.(*RRSIG)}, e, s
>>>>>>> 85acc1406... Bump K8s libraries to 1.13.4
	}
	rr.Serial = uint32(j)

<<<<<<< HEAD
	c.Next() // zBlank

	l, _ = c.Next()
	j, e = strconv.ParseUint(l.token, 10, 16)
	if e != nil {
		// Serial must be a number
		return &ParseError{"", "bad CSYNC flags", l}
	}
	rr.Flags = uint16(j)

	rr.TypeBitMap = make([]uint16, 0)
	var (
		k  uint16
		ok bool
	)
	l, _ = c.Next()
	for l.value != zNewline && l.value != zEOF {
		switch l.value {
		case zBlank:
			// Ok
		case zString:
			tokenUpper := strings.ToUpper(l.token)
			if k, ok = StringToType[tokenUpper]; !ok {
				if k, ok = typeToInt(l.token); !ok {
					return &ParseError{"", "bad CSYNC TypeBitMap", l}
				}
			}
			rr.TypeBitMap = append(rr.TypeBitMap, k)
		default:
			return &ParseError{"", "bad CSYNC TypeBitMap", l}
		}
		l, _ = c.Next()
	}
	return nil
}

func (rr *SIG) parse(c *zlexer, o string) *ParseError {
	return rr.RRSIG.parse(c, o)
}

func (rr *RRSIG) parse(c *zlexer, o string) *ParseError {
	l, _ := c.Next()
	tokenUpper := strings.ToUpper(l.token)
	if t, ok := StringToType[tokenUpper]; !ok {
		if strings.HasPrefix(tokenUpper, "TYPE") {
			t, ok = typeToInt(l.token)
=======
func setRRSIG(h RR_Header, c chan lex, o, f string) (RR, *ParseError, string) {
	rr := new(RRSIG)
	rr.Hdr = h

	l := <-c
	if l.length == 0 { // dynamic update rr.
		return rr, nil, l.comment
	}

	if t, ok := StringToType[l.tokenUpper]; !ok {
		if strings.HasPrefix(l.tokenUpper, "TYPE") {
			t, ok = typeToInt(l.tokenUpper)
>>>>>>> 85acc1406... Bump K8s libraries to 1.13.4
			if !ok {
				return &ParseError{"", "bad RRSIG Typecovered", l}
			}
			rr.TypeCovered = t
		} else {
			return &ParseError{"", "bad RRSIG Typecovered", l}
		}
	} else {
		rr.TypeCovered = t
	}

<<<<<<< HEAD
	c.Next() // zBlank
	l, _ = c.Next()
=======
	<-c // zBlank
	l = <-c
>>>>>>> 85acc1406... Bump K8s libraries to 1.13.4
	i, err := strconv.ParseUint(l.token, 10, 8)
	if err != nil || l.err {
		return &ParseError{"", "bad RRSIG Algorithm", l}
	}
	rr.Algorithm = uint8(i)

<<<<<<< HEAD
	c.Next() // zBlank
	l, _ = c.Next()
=======
	<-c // zBlank
	l = <-c
>>>>>>> 85acc1406... Bump K8s libraries to 1.13.4
	i, err = strconv.ParseUint(l.token, 10, 8)
	if err != nil || l.err {
		return &ParseError{"", "bad RRSIG Labels", l}
	}
	rr.Labels = uint8(i)

<<<<<<< HEAD
	c.Next() // zBlank
	l, _ = c.Next()
=======
	<-c // zBlank
	l = <-c
>>>>>>> 85acc1406... Bump K8s libraries to 1.13.4
	i, err = strconv.ParseUint(l.token, 10, 32)
	if err != nil || l.err {
		return &ParseError{"", "bad RRSIG OrigTtl", l}
	}
	rr.OrigTtl = uint32(i)

<<<<<<< HEAD
	c.Next() // zBlank
	l, _ = c.Next()
=======
	<-c // zBlank
	l = <-c
>>>>>>> 85acc1406... Bump K8s libraries to 1.13.4
	if i, err := StringToTime(l.token); err != nil {
		// Try to see if all numeric and use it as epoch
		if i, err := strconv.ParseInt(l.token, 10, 64); err == nil {
			// TODO(miek): error out on > MAX_UINT32, same below
			rr.Expiration = uint32(i)
		} else {
			return &ParseError{"", "bad RRSIG Expiration", l}
		}
	} else {
		rr.Expiration = i
	}

<<<<<<< HEAD
	c.Next() // zBlank
	l, _ = c.Next()
=======
	<-c // zBlank
	l = <-c
>>>>>>> 85acc1406... Bump K8s libraries to 1.13.4
	if i, err := StringToTime(l.token); err != nil {
		if i, err := strconv.ParseInt(l.token, 10, 64); err == nil {
			rr.Inception = uint32(i)
		} else {
			return &ParseError{"", "bad RRSIG Inception", l}
		}
	} else {
		rr.Inception = i
	}

<<<<<<< HEAD
	c.Next() // zBlank
	l, _ = c.Next()
=======
	<-c // zBlank
	l = <-c
>>>>>>> 85acc1406... Bump K8s libraries to 1.13.4
	i, err = strconv.ParseUint(l.token, 10, 16)
	if err != nil || l.err {
		return &ParseError{"", "bad RRSIG KeyTag", l}
	}
	rr.KeyTag = uint16(i)

<<<<<<< HEAD
	c.Next() // zBlank
	l, _ = c.Next()
	rr.SignerName = l.token
	name, nameOk := toAbsoluteName(l.token, o)
	if l.err || !nameOk {
		return &ParseError{"", "bad RRSIG SignerName", l}
	}
	rr.SignerName = name

	s, e := endingToString(c, "bad RRSIG Signature")
=======
	<-c // zBlank
	l = <-c
	rr.SignerName = l.token
	name, nameOk := toAbsoluteName(l.token, o)
	if l.err || !nameOk {
		return nil, &ParseError{f, "bad RRSIG SignerName", l}, ""
	}
	rr.SignerName = name

	s, e, c1 := endingToString(c, "bad RRSIG Signature", f)
>>>>>>> 85acc1406... Bump K8s libraries to 1.13.4
	if e != nil {
		return e
	}
	rr.Signature = s
<<<<<<< HEAD
=======

	return rr, nil, c1
}
>>>>>>> 85acc1406... Bump K8s libraries to 1.13.4

	return nil
}

<<<<<<< HEAD
func (rr *NSEC) parse(c *zlexer, o string) *ParseError {
	l, _ := c.Next()
	name, nameOk := toAbsoluteName(l.token, o)
	if l.err || !nameOk {
		return &ParseError{"", "bad NSEC NextDomain", l}
=======
	l := <-c
	rr.NextDomain = l.token
	if l.length == 0 { // dynamic update rr.
		return rr, nil, l.comment
	}

	name, nameOk := toAbsoluteName(l.token, o)
	if l.err || !nameOk {
		return nil, &ParseError{f, "bad NSEC NextDomain", l}, ""
>>>>>>> 85acc1406... Bump K8s libraries to 1.13.4
	}
	rr.NextDomain = name

	rr.TypeBitMap = make([]uint16, 0)
	var (
		k  uint16
		ok bool
	)
	l, _ = c.Next()
	for l.value != zNewline && l.value != zEOF {
		switch l.value {
		case zBlank:
			// Ok
		case zString:
			tokenUpper := strings.ToUpper(l.token)
			if k, ok = StringToType[tokenUpper]; !ok {
				if k, ok = typeToInt(l.token); !ok {
					return &ParseError{"", "bad NSEC TypeBitMap", l}
				}
			}
			rr.TypeBitMap = append(rr.TypeBitMap, k)
		default:
			return &ParseError{"", "bad NSEC TypeBitMap", l}
		}
		l, _ = c.Next()
	}
	return nil
}

<<<<<<< HEAD
func (rr *NSEC3) parse(c *zlexer, o string) *ParseError {
	l, _ := c.Next()
=======
func setNSEC3(h RR_Header, c chan lex, o, f string) (RR, *ParseError, string) {
	rr := new(NSEC3)
	rr.Hdr = h

	l := <-c
	if l.length == 0 { // dynamic update rr.
		return rr, nil, l.comment
	}

>>>>>>> 85acc1406... Bump K8s libraries to 1.13.4
	i, e := strconv.ParseUint(l.token, 10, 8)
	if e != nil || l.err {
		return &ParseError{"", "bad NSEC3 Hash", l}
	}
	rr.Hash = uint8(i)
<<<<<<< HEAD
	c.Next() // zBlank
	l, _ = c.Next()
=======
	<-c // zBlank
	l = <-c
>>>>>>> 85acc1406... Bump K8s libraries to 1.13.4
	i, e = strconv.ParseUint(l.token, 10, 8)
	if e != nil || l.err {
		return &ParseError{"", "bad NSEC3 Flags", l}
	}
	rr.Flags = uint8(i)
<<<<<<< HEAD
	c.Next() // zBlank
	l, _ = c.Next()
=======
	<-c // zBlank
	l = <-c
>>>>>>> 85acc1406... Bump K8s libraries to 1.13.4
	i, e = strconv.ParseUint(l.token, 10, 16)
	if e != nil || l.err {
		return &ParseError{"", "bad NSEC3 Iterations", l}
	}
	rr.Iterations = uint16(i)
	c.Next()
	l, _ = c.Next()
	if len(l.token) == 0 || l.err {
		return &ParseError{"", "bad NSEC3 Salt", l}
	}
	if l.token != "-" {
		rr.SaltLength = uint8(len(l.token)) / 2
		rr.Salt = l.token
	}

	c.Next()
	l, _ = c.Next()
	if len(l.token) == 0 || l.err {
		return &ParseError{"", "bad NSEC3 NextDomain", l}
	}
	rr.HashLength = 20 // Fix for NSEC3 (sha1 160 bits)
	rr.NextDomain = l.token

	rr.TypeBitMap = make([]uint16, 0)
	var (
		k  uint16
		ok bool
	)
	l, _ = c.Next()
	for l.value != zNewline && l.value != zEOF {
		switch l.value {
		case zBlank:
			// Ok
		case zString:
			tokenUpper := strings.ToUpper(l.token)
			if k, ok = StringToType[tokenUpper]; !ok {
				if k, ok = typeToInt(l.token); !ok {
					return &ParseError{"", "bad NSEC3 TypeBitMap", l}
				}
			}
			rr.TypeBitMap = append(rr.TypeBitMap, k)
		default:
			return &ParseError{"", "bad NSEC3 TypeBitMap", l}
		}
		l, _ = c.Next()
	}
	return nil
}

<<<<<<< HEAD
func (rr *NSEC3PARAM) parse(c *zlexer, o string) *ParseError {
	l, _ := c.Next()
=======
func setNSEC3PARAM(h RR_Header, c chan lex, o, f string) (RR, *ParseError, string) {
	rr := new(NSEC3PARAM)
	rr.Hdr = h

	l := <-c
	if l.length == 0 { // dynamic update rr.
		return rr, nil, ""
	}

>>>>>>> 85acc1406... Bump K8s libraries to 1.13.4
	i, e := strconv.ParseUint(l.token, 10, 8)
	if e != nil || l.err {
		return &ParseError{"", "bad NSEC3PARAM Hash", l}
	}
	rr.Hash = uint8(i)
<<<<<<< HEAD
	c.Next() // zBlank
	l, _ = c.Next()
=======
	<-c // zBlank
	l = <-c
>>>>>>> 85acc1406... Bump K8s libraries to 1.13.4
	i, e = strconv.ParseUint(l.token, 10, 8)
	if e != nil || l.err {
		return &ParseError{"", "bad NSEC3PARAM Flags", l}
	}
	rr.Flags = uint8(i)
<<<<<<< HEAD
	c.Next() // zBlank
	l, _ = c.Next()
=======
	<-c // zBlank
	l = <-c
>>>>>>> 85acc1406... Bump K8s libraries to 1.13.4
	i, e = strconv.ParseUint(l.token, 10, 16)
	if e != nil || l.err {
		return &ParseError{"", "bad NSEC3PARAM Iterations", l}
	}
	rr.Iterations = uint16(i)
	c.Next()
	l, _ = c.Next()
	if l.token != "-" {
		rr.SaltLength = uint8(len(l.token))
		rr.Salt = l.token
	}
	return slurpRemainder(c)
}

<<<<<<< HEAD
func (rr *EUI48) parse(c *zlexer, o string) *ParseError {
	l, _ := c.Next()
	if len(l.token) != 17 || l.err {
		return &ParseError{"", "bad EUI48 Address", l}
=======
func setEUI48(h RR_Header, c chan lex, o, f string) (RR, *ParseError, string) {
	rr := new(EUI48)
	rr.Hdr = h

	l := <-c
	if l.length == 0 { // dynamic update rr.
		return rr, nil, ""
	}

	if l.length != 17 || l.err {
		return nil, &ParseError{f, "bad EUI48 Address", l}, ""
>>>>>>> 85acc1406... Bump K8s libraries to 1.13.4
	}
	addr := make([]byte, 12)
	dash := 0
	for i := 0; i < 10; i += 2 {
		addr[i] = l.token[i+dash]
		addr[i+1] = l.token[i+1+dash]
		dash++
		if l.token[i+1+dash] != '-' {
			return &ParseError{"", "bad EUI48 Address", l}
		}
	}
	addr[10] = l.token[15]
	addr[11] = l.token[16]

	i, e := strconv.ParseUint(string(addr), 16, 48)
	if e != nil {
		return &ParseError{"", "bad EUI48 Address", l}
	}
	rr.Address = i
	return slurpRemainder(c)
}

<<<<<<< HEAD
func (rr *EUI64) parse(c *zlexer, o string) *ParseError {
	l, _ := c.Next()
	if len(l.token) != 23 || l.err {
		return &ParseError{"", "bad EUI64 Address", l}
=======
func setEUI64(h RR_Header, c chan lex, o, f string) (RR, *ParseError, string) {
	rr := new(EUI64)
	rr.Hdr = h

	l := <-c
	if l.length == 0 { // dynamic update rr.
		return rr, nil, ""
	}

	if l.length != 23 || l.err {
		return nil, &ParseError{f, "bad EUI64 Address", l}, ""
>>>>>>> 85acc1406... Bump K8s libraries to 1.13.4
	}
	addr := make([]byte, 16)
	dash := 0
	for i := 0; i < 14; i += 2 {
		addr[i] = l.token[i+dash]
		addr[i+1] = l.token[i+1+dash]
		dash++
		if l.token[i+1+dash] != '-' {
			return &ParseError{"", "bad EUI64 Address", l}
		}
	}
	addr[14] = l.token[21]
	addr[15] = l.token[22]

	i, e := strconv.ParseUint(string(addr), 16, 64)
	if e != nil {
		return &ParseError{"", "bad EUI68 Address", l}
	}
	rr.Address = i
	return slurpRemainder(c)
}

<<<<<<< HEAD
func (rr *SSHFP) parse(c *zlexer, o string) *ParseError {
	l, _ := c.Next()
=======
func setSSHFP(h RR_Header, c chan lex, o, f string) (RR, *ParseError, string) {
	rr := new(SSHFP)
	rr.Hdr = h

	l := <-c
	if l.length == 0 { // dynamic update rr.
		return rr, nil, ""
	}

>>>>>>> 85acc1406... Bump K8s libraries to 1.13.4
	i, e := strconv.ParseUint(l.token, 10, 8)
	if e != nil || l.err {
		return &ParseError{"", "bad SSHFP Algorithm", l}
	}
	rr.Algorithm = uint8(i)
<<<<<<< HEAD
	c.Next() // zBlank
	l, _ = c.Next()
=======
	<-c // zBlank
	l = <-c
>>>>>>> 85acc1406... Bump K8s libraries to 1.13.4
	i, e = strconv.ParseUint(l.token, 10, 8)
	if e != nil || l.err {
		return &ParseError{"", "bad SSHFP Type", l}
	}
	rr.Type = uint8(i)
	c.Next() // zBlank
	s, e1 := endingToString(c, "bad SSHFP Fingerprint")
	if e1 != nil {
		return e1
	}
	rr.FingerPrint = s
	return nil
}

<<<<<<< HEAD
func (rr *DNSKEY) parseDNSKEY(c *zlexer, o, typ string) *ParseError {
	l, _ := c.Next()
=======
func setDNSKEYs(h RR_Header, c chan lex, o, f, typ string) (RR, *ParseError, string) {
	rr := new(DNSKEY)
	rr.Hdr = h

	l := <-c
	if l.length == 0 { // dynamic update rr.
		return rr, nil, l.comment
	}

>>>>>>> 85acc1406... Bump K8s libraries to 1.13.4
	i, e := strconv.ParseUint(l.token, 10, 16)
	if e != nil || l.err {
		return &ParseError{"", "bad " + typ + " Flags", l}
	}
	rr.Flags = uint16(i)
<<<<<<< HEAD
	c.Next()        // zBlank
	l, _ = c.Next() // zString
=======
	<-c     // zBlank
	l = <-c // zString
>>>>>>> 85acc1406... Bump K8s libraries to 1.13.4
	i, e = strconv.ParseUint(l.token, 10, 8)
	if e != nil || l.err {
		return &ParseError{"", "bad " + typ + " Protocol", l}
	}
	rr.Protocol = uint8(i)
<<<<<<< HEAD
	c.Next()        // zBlank
	l, _ = c.Next() // zString
=======
	<-c     // zBlank
	l = <-c // zString
>>>>>>> 85acc1406... Bump K8s libraries to 1.13.4
	i, e = strconv.ParseUint(l.token, 10, 8)
	if e != nil || l.err {
		return &ParseError{"", "bad " + typ + " Algorithm", l}
	}
	rr.Algorithm = uint8(i)
	s, e1 := endingToString(c, "bad "+typ+" PublicKey")
	if e1 != nil {
		return e1
	}
	rr.PublicKey = s
	return nil
}

func (rr *DNSKEY) parse(c *zlexer, o string) *ParseError {
	return rr.parseDNSKEY(c, o, "DNSKEY")
}

func (rr *KEY) parse(c *zlexer, o string) *ParseError {
	return rr.parseDNSKEY(c, o, "KEY")
}

func (rr *CDNSKEY) parse(c *zlexer, o string) *ParseError {
	return rr.parseDNSKEY(c, o, "CDNSKEY")
}

<<<<<<< HEAD
func (rr *RKEY) parse(c *zlexer, o string) *ParseError {
	l, _ := c.Next()
=======
func setRKEY(h RR_Header, c chan lex, o, f string) (RR, *ParseError, string) {
	rr := new(RKEY)
	rr.Hdr = h

	l := <-c
	if l.length == 0 { // dynamic update rr.
		return rr, nil, l.comment
	}

>>>>>>> 85acc1406... Bump K8s libraries to 1.13.4
	i, e := strconv.ParseUint(l.token, 10, 16)
	if e != nil || l.err {
		return &ParseError{"", "bad RKEY Flags", l}
	}
	rr.Flags = uint16(i)
<<<<<<< HEAD
	c.Next()        // zBlank
	l, _ = c.Next() // zString
=======
	<-c     // zBlank
	l = <-c // zString
>>>>>>> 85acc1406... Bump K8s libraries to 1.13.4
	i, e = strconv.ParseUint(l.token, 10, 8)
	if e != nil || l.err {
		return &ParseError{"", "bad RKEY Protocol", l}
	}
	rr.Protocol = uint8(i)
<<<<<<< HEAD
	c.Next()        // zBlank
	l, _ = c.Next() // zString
=======
	<-c     // zBlank
	l = <-c // zString
>>>>>>> 85acc1406... Bump K8s libraries to 1.13.4
	i, e = strconv.ParseUint(l.token, 10, 8)
	if e != nil || l.err {
		return &ParseError{"", "bad RKEY Algorithm", l}
	}
	rr.Algorithm = uint8(i)
	s, e1 := endingToString(c, "bad RKEY PublicKey")
	if e1 != nil {
		return e1
	}
	rr.PublicKey = s
	return nil
}

func (rr *EID) parse(c *zlexer, o string) *ParseError {
	s, e := endingToString(c, "bad EID Endpoint")
	if e != nil {
		return e
	}
	rr.Endpoint = s
	return nil
}

func (rr *NIMLOC) parse(c *zlexer, o string) *ParseError {
	s, e := endingToString(c, "bad NIMLOC Locator")
	if e != nil {
		return e
	}
	rr.Locator = s
	return nil
}

<<<<<<< HEAD
func (rr *GPOS) parse(c *zlexer, o string) *ParseError {
	l, _ := c.Next()
=======
func setGPOS(h RR_Header, c chan lex, o, f string) (RR, *ParseError, string) {
	rr := new(GPOS)
	rr.Hdr = h

	l := <-c
	if l.length == 0 { // dynamic update rr.
		return rr, nil, ""
	}

>>>>>>> 85acc1406... Bump K8s libraries to 1.13.4
	_, e := strconv.ParseFloat(l.token, 64)
	if e != nil || l.err {
		return &ParseError{"", "bad GPOS Longitude", l}
	}
	rr.Longitude = l.token
	c.Next() // zBlank
	l, _ = c.Next()
	_, e = strconv.ParseFloat(l.token, 64)
	if e != nil || l.err {
		return &ParseError{"", "bad GPOS Latitude", l}
	}
	rr.Latitude = l.token
	c.Next() // zBlank
	l, _ = c.Next()
	_, e = strconv.ParseFloat(l.token, 64)
	if e != nil || l.err {
		return &ParseError{"", "bad GPOS Altitude", l}
	}
	rr.Altitude = l.token
	return slurpRemainder(c)
}

<<<<<<< HEAD
func (rr *DS) parseDS(c *zlexer, o, typ string) *ParseError {
	l, _ := c.Next()
=======
func setDSs(h RR_Header, c chan lex, o, f, typ string) (RR, *ParseError, string) {
	rr := new(DS)
	rr.Hdr = h

	l := <-c
	if l.length == 0 { // dynamic update rr.
		return rr, nil, l.comment
	}

>>>>>>> 85acc1406... Bump K8s libraries to 1.13.4
	i, e := strconv.ParseUint(l.token, 10, 16)
	if e != nil || l.err {
		return &ParseError{"", "bad " + typ + " KeyTag", l}
	}
	rr.KeyTag = uint16(i)
<<<<<<< HEAD
	c.Next() // zBlank
	l, _ = c.Next()
	if i, e = strconv.ParseUint(l.token, 10, 8); e != nil {
		tokenUpper := strings.ToUpper(l.token)
		i, ok := StringToAlgorithm[tokenUpper]
=======
	<-c // zBlank
	l = <-c
	if i, e = strconv.ParseUint(l.token, 10, 8); e != nil {
		i, ok := StringToAlgorithm[l.tokenUpper]
>>>>>>> 85acc1406... Bump K8s libraries to 1.13.4
		if !ok || l.err {
			return &ParseError{"", "bad " + typ + " Algorithm", l}
		}
		rr.Algorithm = i
	} else {
		rr.Algorithm = uint8(i)
	}
<<<<<<< HEAD
	c.Next() // zBlank
	l, _ = c.Next()
=======
	<-c // zBlank
	l = <-c
>>>>>>> 85acc1406... Bump K8s libraries to 1.13.4
	i, e = strconv.ParseUint(l.token, 10, 8)
	if e != nil || l.err {
		return &ParseError{"", "bad " + typ + " DigestType", l}
	}
	rr.DigestType = uint8(i)
	s, e1 := endingToString(c, "bad "+typ+" Digest")
	if e1 != nil {
		return e1
	}
	rr.Digest = s
	return nil
}

func (rr *DS) parse(c *zlexer, o string) *ParseError {
	return rr.parseDS(c, o, "DS")
}

func (rr *DLV) parse(c *zlexer, o string) *ParseError {
	return rr.parseDS(c, o, "DLV")
}

func (rr *CDS) parse(c *zlexer, o string) *ParseError {
	return rr.parseDS(c, o, "CDS")
}

<<<<<<< HEAD
func (rr *TA) parse(c *zlexer, o string) *ParseError {
	l, _ := c.Next()
=======
func setTA(h RR_Header, c chan lex, o, f string) (RR, *ParseError, string) {
	rr := new(TA)
	rr.Hdr = h

	l := <-c
	if l.length == 0 { // dynamic update rr.
		return rr, nil, l.comment
	}

>>>>>>> 85acc1406... Bump K8s libraries to 1.13.4
	i, e := strconv.ParseUint(l.token, 10, 16)
	if e != nil || l.err {
		return &ParseError{"", "bad TA KeyTag", l}
	}
	rr.KeyTag = uint16(i)
<<<<<<< HEAD
	c.Next() // zBlank
	l, _ = c.Next()
	if i, e := strconv.ParseUint(l.token, 10, 8); e != nil {
		tokenUpper := strings.ToUpper(l.token)
		i, ok := StringToAlgorithm[tokenUpper]
=======
	<-c // zBlank
	l = <-c
	if i, e := strconv.ParseUint(l.token, 10, 8); e != nil {
		i, ok := StringToAlgorithm[l.tokenUpper]
>>>>>>> 85acc1406... Bump K8s libraries to 1.13.4
		if !ok || l.err {
			return &ParseError{"", "bad TA Algorithm", l}
		}
		rr.Algorithm = i
	} else {
		rr.Algorithm = uint8(i)
	}
<<<<<<< HEAD
	c.Next() // zBlank
	l, _ = c.Next()
=======
	<-c // zBlank
	l = <-c
>>>>>>> 85acc1406... Bump K8s libraries to 1.13.4
	i, e = strconv.ParseUint(l.token, 10, 8)
	if e != nil || l.err {
		return &ParseError{"", "bad TA DigestType", l}
	}
	rr.DigestType = uint8(i)
	s, err := endingToString(c, "bad TA Digest")
	if err != nil {
		return err
	}
	rr.Digest = s
	return nil
}

<<<<<<< HEAD
func (rr *TLSA) parse(c *zlexer, o string) *ParseError {
	l, _ := c.Next()
	i, e := strconv.ParseUint(l.token, 10, 8)
	if e != nil || l.err {
		return &ParseError{"", "bad TLSA Usage", l}
	}
	rr.Usage = uint8(i)
	c.Next() // zBlank
	l, _ = c.Next()
	i, e = strconv.ParseUint(l.token, 10, 8)
=======
func setTLSA(h RR_Header, c chan lex, o, f string) (RR, *ParseError, string) {
	rr := new(TLSA)
	rr.Hdr = h

	l := <-c
	if l.length == 0 { // dynamic update rr.
		return rr, nil, l.comment
	}

	i, e := strconv.ParseUint(l.token, 10, 8)
>>>>>>> 85acc1406... Bump K8s libraries to 1.13.4
	if e != nil || l.err {
		return &ParseError{"", "bad TLSA Selector", l}
	}
	rr.Selector = uint8(i)
	c.Next() // zBlank
	l, _ = c.Next()
	i, e = strconv.ParseUint(l.token, 10, 8)
	if e != nil || l.err {
		return &ParseError{"", "bad TLSA MatchingType", l}
	}
	rr.MatchingType = uint8(i)
	// So this needs be e2 (i.e. different than e), because...??t
	s, e2 := endingToString(c, "bad TLSA Certificate")
	if e2 != nil {
		return e2
	}
	rr.Certificate = s
	return nil
}

func (rr *SMIMEA) parse(c *zlexer, o string) *ParseError {
	l, _ := c.Next()
	i, e := strconv.ParseUint(l.token, 10, 8)
	if e != nil || l.err {
		return &ParseError{"", "bad SMIMEA Usage", l}
	}
	rr.Usage = uint8(i)
<<<<<<< HEAD
	c.Next() // zBlank
	l, _ = c.Next()
=======
	<-c // zBlank
	l = <-c
>>>>>>> 85acc1406... Bump K8s libraries to 1.13.4
	i, e = strconv.ParseUint(l.token, 10, 8)
	if e != nil || l.err {
		return &ParseError{"", "bad SMIMEA Selector", l}
	}
	rr.Selector = uint8(i)
<<<<<<< HEAD
	c.Next() // zBlank
	l, _ = c.Next()
=======
	<-c // zBlank
	l = <-c
>>>>>>> 85acc1406... Bump K8s libraries to 1.13.4
	i, e = strconv.ParseUint(l.token, 10, 8)
	if e != nil || l.err {
		return &ParseError{"", "bad SMIMEA MatchingType", l}
	}
	rr.MatchingType = uint8(i)
	// So this needs be e2 (i.e. different than e), because...??t
	s, e2 := endingToString(c, "bad SMIMEA Certificate")
	if e2 != nil {
		return e2
	}
	rr.Certificate = s
	return nil
}

<<<<<<< HEAD
func (rr *RFC3597) parse(c *zlexer, o string) *ParseError {
	l, _ := c.Next()
=======
func setSMIMEA(h RR_Header, c chan lex, o, f string) (RR, *ParseError, string) {
	rr := new(SMIMEA)
	rr.Hdr = h

	l := <-c
	if l.length == 0 { // dynamic update rr.
		return rr, nil, l.comment
	}

	i, e := strconv.ParseUint(l.token, 10, 8)
	if e != nil || l.err {
		return nil, &ParseError{f, "bad SMIMEA Usage", l}, ""
	}
	rr.Usage = uint8(i)
	<-c // zBlank
	l = <-c
	i, e = strconv.ParseUint(l.token, 10, 8)
	if e != nil || l.err {
		return nil, &ParseError{f, "bad SMIMEA Selector", l}, ""
	}
	rr.Selector = uint8(i)
	<-c // zBlank
	l = <-c
	i, e = strconv.ParseUint(l.token, 10, 8)
	if e != nil || l.err {
		return nil, &ParseError{f, "bad SMIMEA MatchingType", l}, ""
	}
	rr.MatchingType = uint8(i)
	// So this needs be e2 (i.e. different than e), because...??t
	s, e2, c1 := endingToString(c, "bad SMIMEA Certificate", f)
	if e2 != nil {
		return nil, e2, c1
	}
	rr.Certificate = s
	return rr, nil, c1
}

func setRFC3597(h RR_Header, c chan lex, o, f string) (RR, *ParseError, string) {
	rr := new(RFC3597)
	rr.Hdr = h

	l := <-c
>>>>>>> 85acc1406... Bump K8s libraries to 1.13.4
	if l.token != "\\#" {
		return &ParseError{"", "bad RFC3597 Rdata", l}
	}

<<<<<<< HEAD
	c.Next() // zBlank
	l, _ = c.Next()
=======
	<-c // zBlank
	l = <-c
>>>>>>> 85acc1406... Bump K8s libraries to 1.13.4
	rdlength, e := strconv.Atoi(l.token)
	if e != nil || l.err {
		return &ParseError{"", "bad RFC3597 Rdata ", l}
	}

	s, e1 := endingToString(c, "bad RFC3597 Rdata")
	if e1 != nil {
		return e1
	}
	if rdlength*2 != len(s) {
		return &ParseError{"", "bad RFC3597 Rdata", l}
	}
	rr.Rdata = s
	return nil
}

func (rr *SPF) parse(c *zlexer, o string) *ParseError {
	s, e := endingToTxtSlice(c, "bad SPF Txt")
	if e != nil {
		return e
	}
	rr.Txt = s
	return nil
}

<<<<<<< HEAD
func (rr *AVC) parse(c *zlexer, o string) *ParseError {
	s, e := endingToTxtSlice(c, "bad AVC Txt")
	if e != nil {
		return e
	}
	rr.Txt = s
	return nil
}
=======
func setAVC(h RR_Header, c chan lex, o, f string) (RR, *ParseError, string) {
	rr := new(AVC)
	rr.Hdr = h

	s, e, c1 := endingToTxtSlice(c, "bad AVC Txt", f)
	if e != nil {
		return nil, e, ""
	}
	rr.Txt = s
	return rr, nil, c1
}

func setTXT(h RR_Header, c chan lex, o, f string) (RR, *ParseError, string) {
	rr := new(TXT)
	rr.Hdr = h
>>>>>>> 85acc1406... Bump K8s libraries to 1.13.4

func (rr *TXT) parse(c *zlexer, o string) *ParseError {
	// no zBlank reading here, because all this rdata is TXT
	s, e := endingToTxtSlice(c, "bad TXT Txt")
	if e != nil {
		return e
	}
	rr.Txt = s
	return nil
}

// identical to setTXT
func (rr *NINFO) parse(c *zlexer, o string) *ParseError {
	s, e := endingToTxtSlice(c, "bad NINFO ZSData")
	if e != nil {
		return e
	}
	rr.ZSData = s
	return nil
}

<<<<<<< HEAD
func (rr *URI) parse(c *zlexer, o string) *ParseError {
	l, _ := c.Next()
=======
func setURI(h RR_Header, c chan lex, o, f string) (RR, *ParseError, string) {
	rr := new(URI)
	rr.Hdr = h

	l := <-c
	if l.length == 0 { // dynamic update rr.
		return rr, nil, ""
	}

>>>>>>> 85acc1406... Bump K8s libraries to 1.13.4
	i, e := strconv.ParseUint(l.token, 10, 16)
	if e != nil || l.err {
		return &ParseError{"", "bad URI Priority", l}
	}
	rr.Priority = uint16(i)
<<<<<<< HEAD
	c.Next() // zBlank
	l, _ = c.Next()
=======
	<-c // zBlank
	l = <-c
>>>>>>> 85acc1406... Bump K8s libraries to 1.13.4
	i, e = strconv.ParseUint(l.token, 10, 16)
	if e != nil || l.err {
		return &ParseError{"", "bad URI Weight", l}
	}
	rr.Weight = uint16(i)

	c.Next() // zBlank
	s, err := endingToTxtSlice(c, "bad URI Target")
	if err != nil {
		return err
	}
	if len(s) != 1 {
<<<<<<< HEAD
		return &ParseError{"", "bad URI Target", l}
=======
		return nil, &ParseError{f, "bad URI Target", l}, ""
>>>>>>> 85acc1406... Bump K8s libraries to 1.13.4
	}
	rr.Target = s[0]
	return nil
}

func (rr *DHCID) parse(c *zlexer, o string) *ParseError {
	// awesome record to parse!
	s, e := endingToString(c, "bad DHCID Digest")
	if e != nil {
		return e
	}
	rr.Digest = s
	return nil
}

<<<<<<< HEAD
func (rr *NID) parse(c *zlexer, o string) *ParseError {
	l, _ := c.Next()
=======
func setNID(h RR_Header, c chan lex, o, f string) (RR, *ParseError, string) {
	rr := new(NID)
	rr.Hdr = h

	l := <-c
	if l.length == 0 { // dynamic update rr.
		return rr, nil, ""
	}

>>>>>>> 85acc1406... Bump K8s libraries to 1.13.4
	i, e := strconv.ParseUint(l.token, 10, 16)
	if e != nil || l.err {
		return &ParseError{"", "bad NID Preference", l}
	}
	rr.Preference = uint16(i)
	c.Next()        // zBlank
	l, _ = c.Next() // zString
	u, err := stringToNodeID(l)
	if err != nil || l.err {
		return err
	}
	rr.NodeID = u
	return slurpRemainder(c)
}

<<<<<<< HEAD
func (rr *L32) parse(c *zlexer, o string) *ParseError {
	l, _ := c.Next()
=======
func setL32(h RR_Header, c chan lex, o, f string) (RR, *ParseError, string) {
	rr := new(L32)
	rr.Hdr = h

	l := <-c
	if l.length == 0 { // dynamic update rr.
		return rr, nil, ""
	}

>>>>>>> 85acc1406... Bump K8s libraries to 1.13.4
	i, e := strconv.ParseUint(l.token, 10, 16)
	if e != nil || l.err {
		return &ParseError{"", "bad L32 Preference", l}
	}
	rr.Preference = uint16(i)
	c.Next()        // zBlank
	l, _ = c.Next() // zString
	rr.Locator32 = net.ParseIP(l.token)
	if rr.Locator32 == nil || l.err {
		return &ParseError{"", "bad L32 Locator", l}
	}
	return slurpRemainder(c)
}

<<<<<<< HEAD
func (rr *LP) parse(c *zlexer, o string) *ParseError {
	l, _ := c.Next()
=======
func setLP(h RR_Header, c chan lex, o, f string) (RR, *ParseError, string) {
	rr := new(LP)
	rr.Hdr = h

	l := <-c
	if l.length == 0 { // dynamic update rr.
		return rr, nil, ""
	}

>>>>>>> 85acc1406... Bump K8s libraries to 1.13.4
	i, e := strconv.ParseUint(l.token, 10, 16)
	if e != nil || l.err {
		return &ParseError{"", "bad LP Preference", l}
	}
	rr.Preference = uint16(i)

<<<<<<< HEAD
	c.Next()        // zBlank
	l, _ = c.Next() // zString
	rr.Fqdn = l.token
	name, nameOk := toAbsoluteName(l.token, o)
	if l.err || !nameOk {
		return &ParseError{"", "bad LP Fqdn", l}
	}
	rr.Fqdn = name
=======
	<-c     // zBlank
	l = <-c // zString
	rr.Fqdn = l.token
	name, nameOk := toAbsoluteName(l.token, o)
	if l.err || !nameOk {
		return nil, &ParseError{f, "bad LP Fqdn", l}, ""
	}
	rr.Fqdn = name

	return rr, nil, ""
}
>>>>>>> 85acc1406... Bump K8s libraries to 1.13.4

	return slurpRemainder(c)
}

<<<<<<< HEAD
func (rr *L64) parse(c *zlexer, o string) *ParseError {
	l, _ := c.Next()
=======
	l := <-c
	if l.length == 0 { // dynamic update rr.
		return rr, nil, ""
	}

>>>>>>> 85acc1406... Bump K8s libraries to 1.13.4
	i, e := strconv.ParseUint(l.token, 10, 16)
	if e != nil || l.err {
		return &ParseError{"", "bad L64 Preference", l}
	}
	rr.Preference = uint16(i)
	c.Next()        // zBlank
	l, _ = c.Next() // zString
	u, err := stringToNodeID(l)
	if err != nil || l.err {
		return err
	}
	rr.Locator64 = u
	return slurpRemainder(c)
}

<<<<<<< HEAD
func (rr *UID) parse(c *zlexer, o string) *ParseError {
	l, _ := c.Next()
=======
func setUID(h RR_Header, c chan lex, o, f string) (RR, *ParseError, string) {
	rr := new(UID)
	rr.Hdr = h

	l := <-c
	if l.length == 0 { // dynamic update rr.
		return rr, nil, ""
	}

>>>>>>> 85acc1406... Bump K8s libraries to 1.13.4
	i, e := strconv.ParseUint(l.token, 10, 32)
	if e != nil || l.err {
		return &ParseError{"", "bad UID Uid", l}
	}
	rr.Uid = uint32(i)
	return slurpRemainder(c)
}

<<<<<<< HEAD
func (rr *GID) parse(c *zlexer, o string) *ParseError {
	l, _ := c.Next()
=======
func setGID(h RR_Header, c chan lex, o, f string) (RR, *ParseError, string) {
	rr := new(GID)
	rr.Hdr = h

	l := <-c
	if l.length == 0 { // dynamic update rr.
		return rr, nil, ""
	}

>>>>>>> 85acc1406... Bump K8s libraries to 1.13.4
	i, e := strconv.ParseUint(l.token, 10, 32)
	if e != nil || l.err {
		return &ParseError{"", "bad GID Gid", l}
	}
	rr.Gid = uint32(i)
	return slurpRemainder(c)
}

<<<<<<< HEAD
func (rr *UINFO) parse(c *zlexer, o string) *ParseError {
	s, e := endingToTxtSlice(c, "bad UINFO Uinfo")
	if e != nil {
		return e
	}
	if ln := len(s); ln == 0 {
		return nil
	}
	rr.Uinfo = s[0] // silently discard anything after the first character-string
	return nil
}

func (rr *PX) parse(c *zlexer, o string) *ParseError {
	l, _ := c.Next()
=======
func setUINFO(h RR_Header, c chan lex, o, f string) (RR, *ParseError, string) {
	rr := new(UINFO)
	rr.Hdr = h

	s, e, c1 := endingToTxtSlice(c, "bad UINFO Uinfo", f)
	if e != nil {
		return nil, e, c1
	}
	if ln := len(s); ln == 0 {
		return rr, nil, c1
	}
	rr.Uinfo = s[0] // silently discard anything after the first character-string
	return rr, nil, c1
}

func setPX(h RR_Header, c chan lex, o, f string) (RR, *ParseError, string) {
	rr := new(PX)
	rr.Hdr = h

	l := <-c
	if l.length == 0 { // dynamic update rr.
		return rr, nil, ""
	}

>>>>>>> 85acc1406... Bump K8s libraries to 1.13.4
	i, e := strconv.ParseUint(l.token, 10, 16)
	if e != nil || l.err {
		return &ParseError{"", "bad PX Preference", l}
	}
	rr.Preference = uint16(i)

<<<<<<< HEAD
	c.Next()        // zBlank
	l, _ = c.Next() // zString
	rr.Map822 = l.token
	map822, map822Ok := toAbsoluteName(l.token, o)
	if l.err || !map822Ok {
		return &ParseError{"", "bad PX Map822", l}
	}
	rr.Map822 = map822

	c.Next()        // zBlank
	l, _ = c.Next() // zString
	rr.Mapx400 = l.token
	mapx400, mapx400Ok := toAbsoluteName(l.token, o)
	if l.err || !mapx400Ok {
		return &ParseError{"", "bad PX Mapx400", l}
	}
	rr.Mapx400 = mapx400

	return slurpRemainder(c)
}

func (rr *CAA) parse(c *zlexer, o string) *ParseError {
	l, _ := c.Next()
=======
	<-c     // zBlank
	l = <-c // zString
	rr.Map822 = l.token
	map822, map822Ok := toAbsoluteName(l.token, o)
	if l.err || !map822Ok {
		return nil, &ParseError{f, "bad PX Map822", l}, ""
	}
	rr.Map822 = map822

	<-c     // zBlank
	l = <-c // zString
	rr.Mapx400 = l.token
	mapx400, mapx400Ok := toAbsoluteName(l.token, o)
	if l.err || !mapx400Ok {
		return nil, &ParseError{f, "bad PX Mapx400", l}, ""
	}
	rr.Mapx400 = mapx400

	return rr, nil, ""
}

func setCAA(h RR_Header, c chan lex, o, f string) (RR, *ParseError, string) {
	rr := new(CAA)
	rr.Hdr = h

	l := <-c
	if l.length == 0 { // dynamic update rr.
		return rr, nil, l.comment
	}

>>>>>>> 85acc1406... Bump K8s libraries to 1.13.4
	i, err := strconv.ParseUint(l.token, 10, 8)
	if err != nil || l.err {
		return &ParseError{"", "bad CAA Flag", l}
	}
	rr.Flag = uint8(i)

	c.Next()        // zBlank
	l, _ = c.Next() // zString
	if l.value != zString {
		return &ParseError{"", "bad CAA Tag", l}
	}
	rr.Tag = l.token

	c.Next() // zBlank
	s, e := endingToTxtSlice(c, "bad CAA Value")
	if e != nil {
		return e
	}
	if len(s) != 1 {
<<<<<<< HEAD
		return &ParseError{"", "bad CAA Value", l}
	}
	rr.Value = s[0]
	return nil
}

func (rr *TKEY) parse(c *zlexer, o string) *ParseError {
	l, _ := c.Next()

	// Algorithm
	if l.value != zString {
		return &ParseError{"", "bad TKEY algorithm", l}
	}
	rr.Algorithm = l.token
	c.Next() // zBlank

	// Get the key length and key values
	l, _ = c.Next()
	i, err := strconv.ParseUint(l.token, 10, 8)
	if err != nil || l.err {
		return &ParseError{"", "bad TKEY key length", l}
	}
	rr.KeySize = uint16(i)
	c.Next() // zBlank
	l, _ = c.Next()
	if l.value != zString {
		return &ParseError{"", "bad TKEY key", l}
	}
	rr.Key = l.token
	c.Next() // zBlank

	// Get the otherdata length and string data
	l, _ = c.Next()
	i, err = strconv.ParseUint(l.token, 10, 8)
	if err != nil || l.err {
		return &ParseError{"", "bad TKEY otherdata length", l}
	}
	rr.OtherLen = uint16(i)
	c.Next() // zBlank
	l, _ = c.Next()
	if l.value != zString {
		return &ParseError{"", "bad TKEY otherday", l}
	}
	rr.OtherData = l.token

	return nil
=======
		return nil, &ParseError{f, "bad CAA Value", l}, ""
	}
	rr.Value = s[0]
	return rr, nil, c1
}

func setTKEY(h RR_Header, c chan lex, o, f string) (RR, *ParseError, string) {
	rr := new(TKEY)
	rr.Hdr = h

	l := <-c

	// Algorithm
	if l.value != zString {
		return nil, &ParseError{f, "bad TKEY algorithm", l}, ""
	}
	rr.Algorithm = l.token
	<-c // zBlank

	// Get the key length and key values
	l = <-c
	i, err := strconv.ParseUint(l.token, 10, 8)
	if err != nil || l.err {
		return nil, &ParseError{f, "bad TKEY key length", l}, ""
	}
	rr.KeySize = uint16(i)
	<-c // zBlank
	l = <-c
	if l.value != zString {
		return nil, &ParseError{f, "bad TKEY key", l}, ""
	}
	rr.Key = l.token
	<-c // zBlank

	// Get the otherdata length and string data
	l = <-c
	i, err = strconv.ParseUint(l.token, 10, 8)
	if err != nil || l.err {
		return nil, &ParseError{f, "bad TKEY otherdata length", l}, ""
	}
	rr.OtherLen = uint16(i)
	<-c // zBlank
	l = <-c
	if l.value != zString {
		return nil, &ParseError{f, "bad TKEY otherday", l}, ""
	}
	rr.OtherData = l.token

	return rr, nil, ""
}

var typeToparserFunc = map[uint16]parserFunc{
	TypeAAAA:       {setAAAA, false},
	TypeAFSDB:      {setAFSDB, false},
	TypeA:          {setA, false},
	TypeCAA:        {setCAA, true},
	TypeCDS:        {setCDS, true},
	TypeCDNSKEY:    {setCDNSKEY, true},
	TypeCERT:       {setCERT, true},
	TypeCNAME:      {setCNAME, false},
	TypeCSYNC:      {setCSYNC, true},
	TypeDHCID:      {setDHCID, true},
	TypeDLV:        {setDLV, true},
	TypeDNAME:      {setDNAME, false},
	TypeKEY:        {setKEY, true},
	TypeDNSKEY:     {setDNSKEY, true},
	TypeDS:         {setDS, true},
	TypeEID:        {setEID, true},
	TypeEUI48:      {setEUI48, false},
	TypeEUI64:      {setEUI64, false},
	TypeGID:        {setGID, false},
	TypeGPOS:       {setGPOS, false},
	TypeHINFO:      {setHINFO, true},
	TypeHIP:        {setHIP, true},
	TypeKX:         {setKX, false},
	TypeL32:        {setL32, false},
	TypeL64:        {setL64, false},
	TypeLOC:        {setLOC, true},
	TypeLP:         {setLP, false},
	TypeMB:         {setMB, false},
	TypeMD:         {setMD, false},
	TypeMF:         {setMF, false},
	TypeMG:         {setMG, false},
	TypeMINFO:      {setMINFO, false},
	TypeMR:         {setMR, false},
	TypeMX:         {setMX, false},
	TypeNAPTR:      {setNAPTR, false},
	TypeNID:        {setNID, false},
	TypeNIMLOC:     {setNIMLOC, true},
	TypeNINFO:      {setNINFO, true},
	TypeNSAPPTR:    {setNSAPPTR, false},
	TypeNSEC3PARAM: {setNSEC3PARAM, false},
	TypeNSEC3:      {setNSEC3, true},
	TypeNSEC:       {setNSEC, true},
	TypeNS:         {setNS, false},
	TypeOPENPGPKEY: {setOPENPGPKEY, true},
	TypePTR:        {setPTR, false},
	TypePX:         {setPX, false},
	TypeSIG:        {setSIG, true},
	TypeRKEY:       {setRKEY, true},
	TypeRP:         {setRP, false},
	TypeRRSIG:      {setRRSIG, true},
	TypeRT:         {setRT, false},
	TypeSMIMEA:     {setSMIMEA, true},
	TypeSOA:        {setSOA, false},
	TypeSPF:        {setSPF, true},
	TypeAVC:        {setAVC, true},
	TypeSRV:        {setSRV, false},
	TypeSSHFP:      {setSSHFP, true},
	TypeTALINK:     {setTALINK, false},
	TypeTA:         {setTA, true},
	TypeTLSA:       {setTLSA, true},
	TypeTXT:        {setTXT, true},
	TypeUID:        {setUID, false},
	TypeUINFO:      {setUINFO, true},
	TypeURI:        {setURI, true},
	TypeX25:        {setX25, false},
	TypeTKEY:       {setTKEY, true},
>>>>>>> 85acc1406... Bump K8s libraries to 1.13.4
}
