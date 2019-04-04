package orange

import (
	"net/http"
	"time"
)

// DefaultQueryTimeout is used when no HTTPClient is provided to control the
// duration a query will remain in flight prior to automatic cancellation.
const DefaultQueryTimeout = 30 * time.Second

// DefaultDialTimeout is used when no HTTPClient is provided to control the
// timeout for establishing a new connection.
const DefaultDialTimeout = 5 * time.Second

// DefaultDialKeepAlive is used when no HTTPClient is provided to control the
// keep-alive duration for an active connection.
const DefaultDialKeepAlive = 30 * time.Second

// DefaultMaxIdleConnsPerHost is used when no HTTPClient is provided to control
// how many idle connections to keep alive per host.
const DefaultMaxIdleConnsPerHost = 1

// Config provides a way to list the range server addresses, and a way to
// override defaults when creating new http.Client instances.
type Config struct {
	// HTTPClient allows the caller to specify a specially configured
	// http.Client instance to use for all queries.  When none is provided, a
	// client will be created using the default timeouts.
	HTTPClient *http.Client

	// RetryCallback is predicate function that tests whether query should be
	// retried for a given error.  Leave nil to retry all errors.
	RetryCallback func(error) bool

	// RetryCount is number of query retries to be issued if query returns
	// error.  Leave 0 to never retry query errors.
	RetryCount int

	// RetryPause is the amount of time to wait before retrying the query.
	RetryPause time.Duration

	// Servers is slice of range server address strings.  Must contain at least
	// one string.
	Servers []string
}
