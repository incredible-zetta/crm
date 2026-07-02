package xclient

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"strings"
)

// Cookie is a single parsed cookie from a Netscape cookie file.
type Cookie struct {
	Domain string
	Path   string
	Secure bool
	Expiry string
	Name   string
	Value  string
}

// CookieJar holds parsed cookies indexed by name.
type CookieJar struct {
	byName map[string]Cookie
	order  []string
}

// ParseNetscapeCookieFile reads a Netscape-format cookie file (as exported by
// browsers / curl) and returns a CookieJar.
func ParseNetscapeCookieFile(path string) (*CookieJar, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("open cookie file: %w", err)
	}
	defer f.Close()

	return ParseNetscapeCookies(f)
}

// ParseNetscapeCookies parses Netscape-format cookie content from any reader.
// Used to build a jar from an in-memory cookie blob (e.g. passed per MCP call
// for a specific account) instead of a file on disk.
func ParseNetscapeCookies(r io.Reader) (*CookieJar, error) {
	jar := &CookieJar{byName: make(map[string]Cookie)}
	sc := bufio.NewScanner(r)
	lineNo := 0
	for sc.Scan() {
		lineNo++
		line := strings.TrimRight(sc.Text(), "\r\n")
		if line == "" {
			continue
		}
		// Netscape format allows "#HttpOnly_" prefix on comment-looking lines.
		trimmed := strings.TrimPrefix(line, "#HttpOnly_")
		if strings.HasPrefix(trimmed, "#") {
			continue
		}
		fields := strings.Split(trimmed, "\t")
		if len(fields) != 7 {
			// Skip malformed lines silently; cookie files often have stray rows.
			continue
		}
		c := Cookie{
			Domain: fields[0],
			Path:   fields[2],
			Secure: strings.EqualFold(fields[3], "TRUE"),
			Expiry: fields[4],
			Name:   fields[5],
			Value:  fields[6],
		}
		if _, seen := jar.byName[c.Name]; !seen {
			jar.order = append(jar.order, c.Name)
		}
		jar.byName[c.Name] = c
	}
	if err := sc.Err(); err != nil {
		return nil, fmt.Errorf("scan cookie file: %w", err)
	}
	return jar, nil
}

// Get returns the value for a cookie name and whether it was found.
func (j *CookieJar) Get(name string) (string, bool) {
	c, ok := j.byName[name]
	return c.Value, ok
}

// MustGet returns the value or an error if the cookie is missing.
func (j *CookieJar) MustGet(name string) (string, error) {
	v, ok := j.Get(name)
	if !ok {
		return "", fmt.Errorf("required cookie %q not found", name)
	}
	return v, nil
}

// Header builds a Cookie header string from all parsed cookies in file order.
func (j *CookieJar) Header() string {
	var b strings.Builder
	for i, name := range j.order {
		if i > 0 {
			b.WriteString("; ")
		}
		b.WriteString(name)
		b.WriteByte('=')
		b.WriteString(j.byName[name].Value)
	}
	return b.String()
}
