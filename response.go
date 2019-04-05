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
// function is provided because Response will not properly work if the final
// byte read is not a newline.  It is necessary to standardize final newline
// because some range server implementations return a final newline and some do
// not.
func newResponseFromReader(r io.Reader) (*Response, error) {
	buf, err := ioutil.ReadAll(r)
	if err != nil {
		return nil, err
	}
	if l := len(buf); l == 0 || buf[l-1] != '\n' {
		buf = append(buf, '\n')
	}
	return &Response{buf: buf}, nil
}

// Bytes returns the slice of bytes from the response.
func (r *Response) Bytes() []byte { return r.buf }

// Split returns a slice of strings, each string representing one line from the
// response.
func (r *Response) Split() []string {
	if len(r.buf) == 1 {
		// Because we only create Response from buffers that end with newlines,
		// when we have a single byte in the buffer, it must be the newline,
		// which means we have no results.
		return nil
	}
	slice := strings.Split(string(r.buf), "\n")
	return slice[:len(slice)-1] // remove the final empty element
}
