package orange

import (
	"strings"
)

// response represents a response from a range query from which either the raw
// bytes or a slice of strings may be obtained.
type response struct {
	buf []byte
}

// newResponse returns a response instance that removes the final byte when it
// is a newline character.
func newResponse(buf []byte) *response {
	// When final byte is newline, trim it.
	if l := len(buf); l > 0 && buf[l-1] == '\n' {
		buf = buf[:l-1]
	}
	return &response{buf: buf}
}

// Bytes returns the slice of bytes from the response.
func (r *response) Bytes() []byte { return r.buf }

// Split returns a slice of strings, each string representing one line from the
// response.
func (r *response) Split() []string {
	if len(r.buf) > 0 {
		return strings.Split(string(r.buf), "\n")
	}
	return nil
}
