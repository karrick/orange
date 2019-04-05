package orange

import (
	"io"
	"io/ioutil"
	"strings"
)

// Response represents a response from a range query from which either the raw
// bytes or a slice of strings may be obtained.
type Response struct {
	buf []byte
}

// newResponseFromReader returns a Response structure after reading the provided
// io.Reader, or an error if reading resulted in an error.  This initialization
// function is provided because some range server implementations return a final
// newline and some do not.  This function normalizes those responses.
func newResponseFromReader(r io.Reader) (*Response, error) {
	buf, err := ioutil.ReadAll(r)
	if err != nil {
		return nil, err
	}
	// When final byte is newline, trim it.
	if l := len(buf); l > 0 && buf[l-1] == '\n' {
		buf = buf[:l-1]
	}
	return &Response{buf: buf}, nil
}

// Bytes returns the slice of bytes from the response.
func (r *Response) Bytes() []byte { return r.buf }

// Split returns a slice of strings, each string representing one line from the
// response.
func (r *Response) Split() []string {
	if len(r.buf) > 0 {
		return strings.Split(string(r.buf), "\n")
	}
	return nil
}
