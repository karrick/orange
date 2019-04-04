package orange

import (
	"bufio"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"
)

// defaultQueryLengthThreshold defines the maximum length of the URI for an
// outgoing GET query.  Queries that require a longer URI will automatically be
// sent out via a PUT query.
const defaultQueryLengthThreshold = 4096

// Client attempts to resolve range queries to a list of strings or an error.
type Client struct {
	// The only thing that prevents us from exposing a structure with all public
	// fields is the fact that we need to create the round robin list of
	// servers, and validate other config parameters.
	httpClient    *http.Client
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
//	func main() {
//		servers := []string{"range1.example.com", "range2.example.com", "range3.example.com"}
//
//		config := &orange.Config{
//			RetryCount:              len(servers),
//			RetryPause:              5 * time.Second,
//			Servers:                 servers,
//		}
//
//		client, err := orange.NewQuerier(config)
//		if err != nil {
//			fmt.Fprintf(os.Stderr, "%s", err)
//			os.Exit(1)
//		}
//	}
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

// Queries sends each query expression out in parallel and returns the set union
// of the responses from each query.
//
//     expressions := []string{"%query1", "%query2"}
//
//     lines, err := client.Queries(expressions)
//     if err != nil {
//         fmt.Fprintf(os.Stderr, "ERROR: %s", err)
//         os.Exit(1)
//     }
//     for _, line := range lines {
//         fmt.Println(line)
//     }
func (c *Client) Queries(expressions []string) ([]string, error) {
	results := make(map[string]struct{})
	var resultsErr error
	var resultsLock sync.Mutex

	var wg sync.WaitGroup
	wg.Add(len(expressions))

	for _, expression := range expressions {
		go func(e string) {
			defer wg.Done()
			lines, err := c.Query(e)
			resultsLock.Lock()
			if err != nil {
				resultsErr = err
			} else {
				for _, line := range lines {
					results[line] = struct{}{}
				}
			}
			resultsLock.Unlock()
		}(expression)
	}

	wg.Wait()

	if resultsErr != nil {
		return nil, resultsErr
	}

	values := make([]string, 0, len(results)) // NOTE: len 0 for append
	for v := range results {
		values = append(values, v)
	}

	return values, nil
}

// Query sends out a single query and returns the results or an error.
//
// The query is sent to one or more of the configured range servers.  If a
// particular query results in an error, the query is retried according to the
// client's RetryCount setting.
//
// If a response includes a RangeException header, it returns ErrRangeException.
// If a query's response HTTP status code is not okay, it returns
// ErrStatusNotOK.  Finally, if a query's HTTP response body cannot be parsed,
// it returns ErrParseException.
//
//     lines, err := client.Query("%someQuery")
//     if err != nil {
//         fmt.Fprintf(os.Stderr, "ERROR: %s", err)
//         os.Exit(1)
//     }
//     for _, line := range lines {
//         fmt.Println(line)
//     }
func (c *Client) Query(expression string) ([]string, error) {
	iorc, err := c.getFromRangeServers(expression)
	if err != nil {
		return nil, err
	}

	// Split input text stream into lines.
	var lines []string

	scanner := bufio.NewScanner(iorc)
	for scanner.Scan() {
		lines = append(lines, scanner.Text())
	}

	err = scanner.Err()  // always check for scan error
	cerr := iorc.Close() // always close the input stream

	// The error returned from scanner has more context of the initial problem
	// than error returned by Close.
	if err != nil {
		return nil, ErrParseException{Err: err}
	}
	if cerr != nil {
		return nil, ErrParseException{Err: cerr}
	}
	return lines, nil
}

// getFromRangeServers iterates through the round robin list of servers, sending
// query to each server, one after the other, until a non-error result is
// obtained.  It returns an io.ReadCloser for reading the HTTP response body, or
// an error when all the servers return an error for that query.
func (c *Client) getFromRangeServers(expression string) (io.ReadCloser, error) {
	var attempts int
	for {
		iorc, err := c.getFromRangeServer(expression)
		if err == nil || attempts == c.retryCount || c.retryCallback(err) == false {
			return iorc, err
		}
		attempts++
		if c.retryPause > 0 {
			time.Sleep(c.retryPause)
		}
	}
}

// getFromRangeServer sends to server the query and returns either a
// io.ReadCloser for reading the valid server response, or an error. This
// function attempts to send the query using both GET and PUT HTTP methods. It
// defaults to using GET first, then trying PUT, unless the query length is
// longer than a program constant, in which case it first tries PUT then will
// try GET.
func (c *Client) getFromRangeServer(expression string) (io.ReadCloser, error) {
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
			response, err = c.httpClient.Get(uri)
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
			return response.Body, nil // range server provided non-error response
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

func (c *Client) putQuery(endpoint, expression string) (*http.Response, error) {
	form := url.Values{"query": []string{expression}}
	request, err := http.NewRequest(http.MethodPut, endpoint, strings.NewReader(form.Encode()))
	if err != nil {
		return nil, err
	}
	request.Header.Add("Content-Type", "application/x-www-form-urlencoded")
	return c.httpClient.Do(request)
}
