package orange

import (
	"fmt"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// defaultQueryLengthThreshold defines the maximum length of the URI for an
// outgoing GET query.  Queries that require a longer URI will automatically be
// sent out via a PUT query.
const defaultQueryLengthThreshold = 4096

// Client provides a Query method that resolves range queries.
type Client struct {
	// The only thing that prevents us from exposing a structure with all public
	// fields is the fact that we need to create the round robin list of
	// servers, and validate other config parameters.
	httpClient    Doer
	servers       *roundRobinStrings
	retryCallback func(error) bool
	retryCount    int
	retryPause    time.Duration
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
			// WARNING: Using http.Client instance without a Timeout will cause resource
			// leaks and may render your program inoperative if the client connects to a
			// buggy range server, or over a poor network connection.
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
	r, err := c.query(expression)
	if err == nil {
		return r.Split(), nil
	}
	return nil, err
}

// QueryBytes sends out a query and returns either a slice of bytes
// corresponding to the HTTP response body received from the range server, or an
// error.
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
//             buf, err := client.QueryBytes(scanner.Text())
//             if err != nil {
//                 fmt.Fprintf(os.Stderr, "ERROR: %s\n", err)
//                 fmt.Printf("> ")
//                 continue
//             }
//             fmt.Println(string(buf))
//         }
//         if err := scanner.Err(); err != nil {
//             fmt.Fprintf(os.Stderr, "ERROR: %s\n", err)
//         }
//     }
func (c *Client) QueryBytes(expression string) ([]byte, error) {
	r, err := c.query(expression)
	if err == nil {
		return r.Bytes(), nil
	}
	return nil, err
}

func (c *Client) query(expression string) (*response, error) {
	// This function iterates through the round robin list of servers, sending
	// query to each server, one after the other, until a non-error result is
	// obtained.  It returns a byte slice from reading the HTTP response body,
	// or an error when all the servers return an error for that query.
	var attempts int
	for {
		results, err := c.getFromRangeServer(expression)
		if err == nil || attempts == c.retryCount || c.retryCallback(err) == false {
			return results, err
		}
		attempts++
		if c.retryPause > 0 {
			time.Sleep(c.retryPause)
		}
	}
}

// getFromRangeServer sends to server the query and returns either a byte slice
// from reading the valid server response, or an error. This function attempts
// to send the query using both GET and PUT HTTP methods. It defaults to using
// GET first, then trying PUT, unless the query length is longer than a program
// constant, in which case it first tries PUT then will try GET.
func (c *Client) getFromRangeServer(expression string) (*response, error) {
	var err, herr error
	var response *http.Response

	// need endpoint for both GET and PUT, so keep it separate
	endpoint := fmt.Sprintf("http://%s/range/list", c.servers.Next())

	// need uri for just GET
	uri := fmt.Sprintf("%s?%s", endpoint, url.QueryEscape(expression))

	// Default to using GET request because most servers support it. However,
	// opt for PUT when extremely long query length.
	var method string
	if len(uri) > defaultQueryLengthThreshold {
		method = http.MethodPut
	} else {
		method = http.MethodGet
	}

	// At least 2 tries so we can try GET or POST if server gives us 405 or 414.
	for triesRemaining := 2; triesRemaining > 0; triesRemaining-- {
		switch method {
		case http.MethodGet:
			response, err = c.getQuery(uri)
		case http.MethodPut:
			response, err = c.putQuery(endpoint, expression)
		default:
			panic(fmt.Errorf("cannot use unsupported HTTP method: %q", method))
		}
		if err != nil {
			return nil, err // could not even make network request
		}

		// Network round trip completed successfully, but there still might be
		// an error condition encoded in the response.

		switch response.StatusCode {
		case http.StatusOK:
			if message := response.Header.Get("RangeException"); message != "" {
				return nil, ErrRangeException{Message: message}
			}
			//
			// NORMAL EXIT PATH: range server provided non-error response
			//
			r, rerr := newResponseFromReader(response.Body)
			cerr := response.Body.Close() // always close regardless of read error
			if rerr != nil {
				return nil, rerr // Read error has more context than Close error
			}
			if cerr != nil {
				return nil, cerr
			}
			return r, nil
		case http.StatusRequestURITooLong:
			method = http.MethodPut // try again using PUT
			herr = ErrStatusNotOK{
				Status:     response.Status,
				StatusCode: response.StatusCode,
			}
		case http.StatusMethodNotAllowed:
			method = http.MethodGet // try again using GET
			herr = ErrStatusNotOK{
				Status:     response.Status,
				StatusCode: response.StatusCode,
			}
		default:
			herr = ErrStatusNotOK{
				Status:     response.Status,
				StatusCode: response.StatusCode,
			}
		}
	}

	return nil, herr
}

func (c *Client) getQuery(url string) (*http.Response, error) {
	request, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	return c.httpClient.Do(request)
}

func (c *Client) putQuery(endpoint, expression string) (*http.Response, error) {
	form := url.Values{"query": []string{expression}}
	request, err := http.NewRequest(http.MethodPut, endpoint, strings.NewReader(form.Encode()))
	if err != nil {
		return nil, err
	}
	request.Header.Add("Content-Type", "application/x-www-form-urlencoded")
	return c.httpClient.Do(request)
}
