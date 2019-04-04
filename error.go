package orange

import (
	"net"
	"net/url"
)

// ErrRangeException is returned when the response includes
// an HTTP 'RangeException' header.
type ErrRangeException struct {
	Message string
}

func (err ErrRangeException) Error() string {
	return "RangeException: " + err.Message
}

// ErrStatusNotOK is returned when the response status code is not Ok.
type ErrStatusNotOK struct {
	Status     string
	StatusCode int
}

func (err ErrStatusNotOK) Error() string {
	return err.Status
}

// ErrParseException is returned by Client.Query method when an error occurs
// while reading the io.ReadCloser from the response.
type ErrParseException struct {
	Err error
}

func (err ErrParseException) Error() string {
	return "cannot parse response: " + err.Err.Error()
}

////////////////////////////////////////
// Some utility functions for the default method of whether or not a query with
// an error result ought to be retried.

type temporary interface {
	Temporary() bool
}

type timeout interface {
	Timeout() bool
}

func isTemporary(err error) bool {
	t, ok := err.(temporary)
	return ok && t.Temporary()
}

func isTimeout(err error) bool {
	t, ok := err.(timeout)
	return ok && t.Timeout()
}

func makeRetryCallback(count int) func(error) bool {
	return func(err error) bool {
		// Because some DNSError errors can be temporary or timeout, most efficient to check
		// whether those conditions are true first.
		if isTemporary(err) || isTimeout(err) {
			return true
		}
		// And if error is neither temporary nor a timeout, then it might still be retryable
		// if it's a DNSError and there are more than one servers configured to proxy for.
		if urlError, ok := err.(*url.Error); ok {
			if netOpError, ok := urlError.Err.(*net.OpError); ok {
				if _, ok = netOpError.Err.(*net.DNSError); ok {
					// "no such host": This query may be retried either if there
					// are more servers in the list of servers, or if the DNS
					// lookup resulted in a timeout.
					return count > 1
				}
			}
		}
		return false
	}
}
