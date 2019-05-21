package orange

import (
	"context"
	"fmt"
	"io"
	"io/ioutil"
	"net"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"
)

// defaultQueryURILengthThreshold defines the maximum length of the URI for an
// outgoing GET query.  Queries that require a longer URI will automatically be
// sent out via a PUT query.
const defaultQueryURILengthThreshold = 4096

// Client provides a Query method that resolves range queries.
type Client struct {
	// The only thing that prevents us from exposing a structure with all public
	// fields is the fact that we need to create the round robin list of
	// servers, and validate other config parameters.
	httpClient        Doer
	servers           *roundRobinStrings
	retryCallback     func(error) bool
	retryCount        int
	retryPause        time.Duration
	verbose, warnings Printer
}

// NewClient returns a new instance that sends queries to one or more range
// servers.  The provided Config not only provides a way of listing one or more
// range servers, but also allows specification of optional retry-on-failure
// features.
//
//     func main() {
//         // Create a range client.  Programs can list more than one server and
//         // include other options.  See Config structure documentation for specifics.
//         client, err := orange.NewClient(&orange.Config{
//             Servers: []string{"localhost:8081"},
//         })
//         if err != nil {
//             fmt.Fprintf(os.Stderr, "%s\n", err)
//             os.Exit(1)
//         }
//
//         // Example program main loop reads query from standard input, queries the
//         // range server, then prints the response.
//         fmt.Printf("> ")
//         scanner := bufio.NewScanner(os.Stdin)
//         for scanner.Scan() {
//             values, err := client.Query(scanner.Text())
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
	if config.RetryCount < 0 {
		return nil, fmt.Errorf("cannot create Querier with negative RetryCount: %d", config.RetryCount)
	}
	if config.RetryPause < 0 {
		return nil, fmt.Errorf("cannot create Querier with negative RetryPause: %s", config.RetryPause)
	}
	rrs, err := newRoundRobinStrings(config.Servers)
	if err != nil {
		return nil, fmt.Errorf("cannot create Querier without at least one range server address")
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
		// verbose:       config.Verbose,
		// warnings:      config.Warnings,
	}

	fmt.Fprintf(os.Stderr, "config.Warnings: %T %p\n", config.Warnings, config.Warnings)
	if config.Verbose != nil {
		fmt.Fprintf(os.Stderr, "config.Verbose: %p\n", config.Verbose)
	}

	return client, nil
}

// Query sends out a query and returns either a slice of strings corresponding
// to the query response or an error.
//
// The query is sent to one or more of the configured range servers.  If a
// particular query results in an error, the query is retried according to the
// client's RetryCount setting.
//
// If a response includes a RangeException header, it returns ErrRangeException.
// If a query's response HTTP status code is not okay, it returns
// ErrStatusNotOK.
//
//     func main() {
//         // Create a range client.  Programs can list more than one server and
//         // include other options.  See Config structure documentation for specifics.
//         client, err := orange.NewClient(&orange.Config{
//             Servers: []string{"localhost:8081"},
//         })
//         if err != nil {
//             fmt.Fprintf(os.Stderr, "%s\n", err)
//             os.Exit(1)
//         }
//
//         // Example program main loop reads query from standard input, queries the
//         // range server, then prints the response.
//         fmt.Printf("> ")
//         scanner := bufio.NewScanner(os.Stdin)
//         for scanner.Scan() {
//             values, err := client.Query(scanner.Text())
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
//         // Create a range client.  Programs can list more than one server and
//         // include other options.  See Config structure documentation for specifics.
//         client, err := orange.NewClient(&orange.Config{
//             Servers: []string{"localhost:8081"},
//         })
//         if err != nil {
//             fmt.Fprintf(os.Stderr, "%s\n", err)
//             os.Exit(1)
//         }
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
//         values, err := client.Query(strings.Join(flag.Args(), ","))
//         if err != nil {
//             fmt.Fprintf(os.Stderr, "ERROR: %s\n", err)
//             os.Exit(1)
//         }
//
//         fmt.Println(values)
//     }
func (c *Client) QueryCtx(ctx context.Context, expression string) ([]string, error) {
	var lines []string

	err := c.QueryCallback(ctx, expression, func(ior io.Reader) error {
		buf, err := ioutil.ReadAll(ior)
		if err != nil {
			return err
		}
		c := len(buf)
		if c == 0 {
			return nil // empty response
		}
		if buf[c-1] == '\n' {
			buf = buf[:c-1] // Trim final byte when it is newline.
			if len(buf) == 0 {
				return nil // nothing left after trimming
			}
		}
		lines = strings.Split(string(buf), "\n")
		return nil
	})

	if err != nil {
		return nil, err
	}

	return lines, nil
}

// QueryCallback sends the query expression to the range client with the
// provided query context.  Upon successful response, invokes specified callback
// function with an io.Reader configured to read the response body from the
// range server.
func (c *Client) QueryCallback(ctx context.Context, expression string, callback func(io.Reader) error) error {
	done := ctx.Done()
	ch := make(chan struct{})
	var err error

	if c.verbose != nil {
		c.verbose.Printf("range query: %s\n", expression)
	}

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
			if err == nil {
				close(ch)
				return
			}

			if c.warnings != nil {
				c.warnings.Printf("FLUBBER: %s\n", err)
			}

			if attempts == c.retryCount || c.retryCallback(err) == false {
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
		return ctx.Err()
	case <-ch:
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
		var startTime time.Time
		if c.verbose != nil {
			startTime = time.Now()
		}

		switch method {
		case http.MethodGet:
			if wasGetTried {
				return prevErr
			}
			wasGetTried = true

			request, err = http.NewRequest(method, uri, nil)
			if err != nil {
				method = http.MethodPut // try again using PUT
				prevErr = err
				continue
			}
		case http.MethodPut:
			if wasPutTried {
				return prevErr
			}
			wasPutTried = true

			request, err = http.NewRequest(method, endpoint, strings.NewReader("query="+escaped))
			if err != nil {
				method = http.MethodGet // try again using GET
				prevErr = err
				continue
			}
			request.Header.Add("Content-Type", "application/x-www-form-urlencoded")
		default:
			panic(fmt.Errorf("this library should not have specified unsupported HTTP method: %q", method))
		}

		// Attach the context and dispatch the request.
		response, err := c.httpClient.Do(request.WithContext(ctx))
		if err != nil {
			return err
		}

		if c.verbose != nil {
			switch method {
			case http.MethodGet:
				c.verbose.Printf("%s %q; latency: %s\n", method, uri, time.Now().Sub(startTime))
			case http.MethodPut:
				c.verbose.Printf("%s %q; latency: %s\n", method, endpoint, time.Now().Sub(startTime))
			}
		}

		// Network request completed successfully, but there still might be an error
		// condition encoded in the response.
		if response.StatusCode == http.StatusOK {
			if message := response.Header.Get("RangeException"); message != "" {
				return ErrRangeException{Message: message}
			}
			//
			// NORMAL EXIT PATH: range server provided non-error response
			//
			prevErr = callback(response.Body)
			err = discard(response.Body)
			if prevErr != nil {
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
			// No more attempts will be made to this target server, so read
			// response body and return its text in the error.
			buf, err := bytesFromReadCloser(response.Body)
			if err == nil && len(buf) > 0 {
				return ErrStatusNotOK{
					Status:     string(buf),
					StatusCode: response.StatusCode,
				}
			}
			return ErrStatusNotOK{
				Status:     response.Status,
				StatusCode: response.StatusCode,
			}
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

func bytesFromReadCloser(iorc io.ReadCloser) ([]byte, error) {
	buf, err1 := ioutil.ReadAll(iorc)
	err2 := iorc.Close()
	if err1 != nil {
		return nil, err1
	}
	if err2 != nil {
		return nil, err2
	}
	return buf, nil
}

func discard(iorc io.ReadCloser) error {
	_, err1 := io.Copy(ioutil.Discard, iorc) // so we can reuse connections via Keep-Alive
	err2 := iorc.Close()
	if err1 != nil {
		return err1
	}
	return err2
}
