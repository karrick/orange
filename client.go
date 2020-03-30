package orange

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"time"
)

// defaultQueryURILengthThreshold defines the maximum length of the URI for an
// outgoing GET query.  Queries that require a longer URI will automatically be
// sent out via a PUT query.
const defaultQueryURILengthThreshold = 4096

const putContentType = "application/x-www-form-urlencoded"

var application, userAgentSuffix string

func init() {
	var err error

	application, err = os.Executable()
	if err != nil {
		application = os.Args[0]
	}
	application = filepath.Base(application)

	account := os.Getenv("LOGNAME")
	if account == "" {
		account = "UNKNOWN"
	}

	hostname, err := os.Hostname()
	if err != nil || hostname == "" {
		hostname = "UNKNOWN"
	}

	userAgentSuffix = fmt.Sprintf("%s@%s via orange", account, hostname)
}

// Client provides a Query method that resolves range queries.
type Client struct {
	stats         Stats // cannot be public; requires atomic access
	userAgent     string
	httpClient    Doer
	servers       *roundRobinStrings // must be initialized
	retryCallback func(error) bool
	retryCount    int
	retryPause    time.Duration
}

// NewClient attempts to create a new range client given the provided
// configuration. The provided Config instance allows the client to specify a
// list of range servers as the sources of truth, but also how to query them,
// and retry handling.
//
//     package main
//
//     import (
//         "bufio"
//         "fmt"
//         "os"
//         "path/filepath"
//
//         "github.com/karrick/orange"
//     )
//
//     var rangeClient *orange.Client
//
//     func init() {
//         var err error
//
//         // Create a range client.  Programs can list more than one server and
//         // include other options.  See Config structure documentation for specifics.
//         rangeClient, err = orange.NewClient(&orange.Config{
//             Servers: []string{"range1", "range2"},
//         })
//         if err != nil {
//             fmt.Fprintf(os.Stderr, "%s: %s\n", filepath.Base(os.Args[0]), err)
//             os.Exit(1)
//         }
//     }
//
//     func main() {
//         // Example program main loop reads query from standard input, queries the
//         // range server, then prints the response.
//         fmt.Printf("> ")
//         scanner := bufio.NewScanner(os.Stdin)
//         for scanner.Scan() {
//             values, err := rangeClient.Query(scanner.Text())
//             if err != nil {
//                 fmt.Fprintf(os.Stderr, "ERROR: %s\n", err)
//                 fmt.Printf("> ")
//                 continue
//             }
//             fmt.Printf("%v\n> ", values)
//         }
//         if err := scanner.Err(); err != nil {
//             fmt.Fprintf(os.Stderr, "ERROR: %s\n", err)
//         }
//     }
func NewClient(config *Config) (*Client, error) {
	if len(config.Servers) == 0 {
		return nil, errors.New("cannot create range client without at least one server")
	}
	if config.RetryCount < 0 {
		return nil, fmt.Errorf("cannot create Client with negative RetryCount: %d", config.RetryCount)
	}
	if config.RetryPause < 0 {
		return nil, fmt.Errorf("cannot create Client with negative RetryPause: %s", config.RetryPause)
	}
	rrs, err := newRoundRobinStrings(config.Servers)
	if err != nil {
		return nil, fmt.Errorf("cannot create Client without at least one range server address")
	}

	retryCallback := config.RetryCallback
	if retryCallback == nil {
		retryCallback = makeRetryCallback(len(config.Servers))
	}

	httpClient := config.HTTPClient
	if httpClient == nil {
		httpClient = &http.Client{
			// WARNING: Using http.Client instance without a Timeout will cause
			// resource leaks and may render your program inoperative if the
			// client connects to a buggy range server, or over a poor network
			// connection.
			Timeout: time.Duration(DefaultQueryTimeout),

			Transport: &http.Transport{
				Dial: (&net.Dialer{
					Timeout:   DefaultDialTimeout,
					KeepAlive: DefaultDialKeepAlive,
				}).Dial,
				MaxIdleConnsPerHost: int(DefaultMaxIdleConnsPerHost),
			},
		}
	}

	client := &Client{
		httpClient:    httpClient,
		retryCallback: retryCallback,
		retryCount:    config.RetryCount,
		retryPause:    config.RetryPause,
		servers:       rrs,
	}

	if config.UserAgent == "" {
		client.userAgent = fmt.Sprintf("%s %s", application, userAgentSuffix)
	} else {
		client.userAgent = fmt.Sprintf("%s %s", config.UserAgent, userAgentSuffix)
	}

	return client, nil
}

// Query sends out a query and returns either a slice of strings corresponding
// to the query response or an error.
//
// The query is sent to one or more of the configured range servers in
// round-robin order.  If a range server returns a temporary network error or
// network timeout error, if there are more than one range servers configured,
// it retries the query with the next configured range server. The query is
// retried according to the client's RetryCount setting.
//
// If a response includes a RangeException header, it returns ErrRangeException.
// If a query's response HTTP status code is not okay, it returns
// ErrStatusNotOK.
//
//     func main() {
//         values, err := rangeClient.Query("%foo.example.1")
//         if err != nil {
//             fmt.Fprintf(os.Stderr, "%s: %s\n", filepath.Base(os.Args[0]), err)
//             os.Exit(1)
//         }
//         fmt.Printf("%v\n> ", values)
//     }
func (c *Client) Query(expression string) ([]string, error) {
	return c.QueryCtx(context.Background(), expression)
}

// QueryCtx sends the query expression to the range client with the provided
// query context.  Callers may opt to use this method when a timeout is required
// for the query.  Note that the shorter timeout applies when using a
// http.Client timeout and a context timeout.  If you intend to only use
// QueryCtx and QueryCallback, then also you might want to pass a different
// HTTPClient argument to the Config so the two timeouts do not cause unexpected
// results.
//
//     func main() {
//         optTimeout := flag.Duration("timeout", 0, "timeout duration for the query")
//         flag.Parse()
//
//         ctx := context.Background()
//         if *optTimeout > 0 {
//             var done func()
//             ctx, done = context.WithTimeout(ctx, *optTimeout)
//             defer done()
//         }
//
//         if flag.NArg() == 0 {
//             fmt.Fprintf(os.Stderr, "USAGE: %s [-timeout DURATION] q1 q2\n")
//             os.Exit(1)
//         }
//
//         rangeClient, err := orange.NewClient(&orange.Config{Servers:[]string{"range"}})
//         if err != nil {
//             fmt.Fprintf(os.Stderr, "%s: %s\n", filepath.Base(os.Args[0]), err)
//             os.Exit(1)
//         }
//
//         values, err := rangeClient.Query(strings.Join(flag.Args(), ","))
//         if err != nil {
//             fmt.Fprintf(os.Stderr, "%s: %s\n", filepath.Base(os.Args[0]), err)
//             os.Exit(1)
//         }
//
//         fmt.Println(values)
//     }
func (c *Client) QueryCtx(ctx context.Context, expression string) (lines []string, err error) {
	err = c.QueryForEach(ctx, expression, func(value string) {
		lines = append(lines, value)
	})
	return
}

// QueryForEach invokes callback for each result value, terminating early
// without processing entire range server response when the provided context is
// closed.
func (c *Client) QueryForEach(ctx context.Context, expression string, callback func(value string)) error {
	return c.QueryCallback(ctx, expression, func(ior io.Reader) error {
		scanner := bufio.NewScanner(ior)
		cDone := ctx.Done()
		sDone := make(chan struct{})

		go func() {
		nextLine:
			if scanner.Scan() {
				select {
				case _ = <-cDone:
					// The context was closed while waiting for input line; do
					// not invoke callback.
				default:
					callback(scanner.Text())
					goto nextLine
				}
			}
			close(sDone)
		}()

		select {
		case _ = <-cDone:
			return ctx.Err()
		case _ = <-sDone:
			return scanner.Err()
		}
	})
}

// QueryCallback sends the query expression to the range client with the
// provided query context.  Upon successful response, invokes specified callback
// function with an io.Reader configured to read the response body from the
// range server.
func (c *Client) QueryCallback(ctx context.Context, expression string, callback func(io.Reader) error) error {
	done := ctx.Done()
	ch := make(chan struct{})
	var err error

	// Spawn a go-routine to send queries to one or more range servers, as
	// allowed by the client's Servers and Retry settings.
	go func() {
		var attempts int

		for {
			// If not first attempt, and there is a retry pause, then wait.
			// This logic will neither sleep on the first attempt nor after the
			// final attempt.
			if attempts > 0 && c.retryPause > 0 {
				time.Sleep(c.retryPause)

				// After wake-up, ensure context has not closed, and return
				// early if it has without sending another query whose results
				// will be simply thrown away.
				select {
				case <-done:
					return
				default:
				}
			}

			err = c.query(ctx, expression, callback, c.servers.Next())
			if err == nil || attempts == c.retryCount || c.retryCallback(err) == false {
				close(ch)
				return
			}

			attempts++
		}
	}()

	// Block and wait for either a response or the context to be closed by the
	// caller.
	select {
	case <-done:
		atomic.AddUint64(&c.stats.ErrContextDone, 1)
		return ctx.Err()
	case <-ch:
		switch err.(type) {
		case nil:
			atomic.AddUint64(&c.stats.NoErr, 1)
		case ErrRangeException:
			atomic.AddUint64(&c.stats.ErrRangeException, 1)
		case ErrStatusNotOK:
			atomic.AddUint64(&c.stats.ErrStatusNotOK, 1)
		default:
			atomic.AddUint64(&c.stats.ErrUnknown, 1)
		}
		return err
	}
}

// query attempts to fetch the results from querying a range server with the
// specified range expression.
//
// It prefers using the GET method when the resulting URI is fewer characters
// than a configured limit, but will re-send the query using the PUT method if
// the range server returns method not allowed response.  When the resulting URI
// is or exceeds a configured limit, it prefers using the PUT method, but will
// re-send the query using the GET method if the range server returns a Method
// Not Allowed,
func (c *Client) query(ctx context.Context, expression string, callback func(io.Reader) error, server string) error {
	var err, prevErr error
	var request *http.Request
	var wasGetTried, wasPutTried bool

	endpoint := "http://" + server + "/range/list"
	escaped := url.QueryEscape(expression)
	uri := endpoint + "?" + escaped

	// Default to using GET method because most servers support it. However, use
	// PUT method when extremely long query length.
	var method string
	if len(uri) > defaultQueryURILengthThreshold {
		method = http.MethodPut
	} else {
		method = http.MethodGet
	}

	for {
		switch method {
		case http.MethodGet:
			if wasGetTried {
				return prevErr
			}
			wasGetTried = true

			request, err = http.NewRequest(method, uri, nil)
			if err != nil {
				prevErr = err
				method = http.MethodPut // try again using PUT
				continue
			}
		case http.MethodPut:
			if wasPutTried {
				return prevErr
			}
			wasPutTried = true

			request, err = http.NewRequest(method, endpoint, strings.NewReader("query="+escaped))
			if err != nil {
				prevErr = err
				method = http.MethodGet // try again using GET
				continue
			}
			request.Header.Set("Content-Type", putContentType)
		default:
			panic(fmt.Errorf("this library should not have specified unsupported HTTP method: %q", method))
		}

		// Attach the context and dispatch the request.
		request.Header.Set("User-Agent", c.userAgent)
		response, err := c.httpClient.Do(request.WithContext(ctx))
		if err != nil {
			return err
		}

		// Network request completed successfully, but there still might be an error
		// condition encoded in the response.
		if response.StatusCode == http.StatusOK {
			if message := response.Header.Get("RangeException"); message != "" {
				e := ErrRangeException{Message: message}

				// Read response body and return its text in the error.
				buf, err := bytesFromReadCloser(response.Body)
				if l := len(buf); err == nil && l > 0 {
					e.Body = buf
				}
				return e
			}
			//
			// NORMAL EXIT PATH: range server provided non-error response
			//
			prevErr = callback(response.Body)
			err = discard(response.Body)
			if prevErr != nil {
				// NOTE: Do not count callback error as ErrUnknown.
				return prevErr
			}
			return err
		}

		switch response.StatusCode {
		case http.StatusRequestURITooLong:
			if wasPutTried {
				return prevErr
			}
			method = http.MethodPut // try again using PUT
		case http.StatusMethodNotAllowed:
			if wasGetTried {
				return prevErr
			}
			method = http.MethodGet // try again using GET
		default:
			e := ErrStatusNotOK{
				Status:     response.Status,
				StatusCode: response.StatusCode,
			}
			// Read response body and return its text in the error.
			buf, err := bytesFromReadCloser(response.Body)
			if l := len(buf); err == nil && l > 0 {
				e.Body = buf
			}
			return e
		}

		// Another attempt is warranted, so discard response body from this attempt,
		// and try again.
		_ = discard(response.Body)

		// Before loop to make another try, abort when context is already done.
		select {
		case <-ctx.Done():
			return ctx.Err() // terminate when client has canceled the context
		default:
			// context still valid: fallthrough and send out a query attempt
			prevErr = err
		}
	}
}

func bytesFromReadCloser(rc io.ReadCloser) ([]byte, error) {
	buf, rerr := ioutil.ReadAll(rc)
	cerr := rc.Close() // always close regardless of read error
	if rerr != nil {
		return buf, rerr // Read error has more context than Close error
	}
	return buf, cerr
}

func discard(iorc io.ReadCloser) error {
	_, err1 := io.Copy(ioutil.Discard, iorc) // so we can reuse connections via Keep-Alive
	err2 := iorc.Close()
	if err1 != nil {
		return err1
	}
	return err2
}

// Stats returns a populated Stats structure and resets the counters.
func (c *Client) Stats() Stats {
	return Stats{
		atomic.SwapUint64(&c.stats.NoErr, 0),
		atomic.SwapUint64(&c.stats.ErrContextDone, 0),
		atomic.SwapUint64(&c.stats.ErrRangeException, 0),
		atomic.SwapUint64(&c.stats.ErrStatusNotOK, 0),
		atomic.SwapUint64(&c.stats.ErrUnknown, 0),
	}
}

// Stats tracks various statistics for the range client.
type Stats struct {
	NoErr             uint64
	ErrContextDone    uint64
	ErrRangeException uint64
	ErrStatusNotOK    uint64
	ErrUnknown        uint64
}
