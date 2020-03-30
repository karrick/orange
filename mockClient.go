package orange

import (
	"io/ioutil"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// MockConfig allows programs to test handling of various types of errors from
// the range client.
type MockConfig struct {
	Results        []string                       // Results allows programmer to specify string slice of results to return.
	Err            error                          // Err allows programmer to force a customized error.
	RangeException string                         // RangeException allows programmer to force return of a RangeException header.
	Callback       func(string) ([]string, error) // Callback, when declared, will be called to provide results and error by the mock client.
	StatusCode     int                            // StatusCode allows programmer to force a non-200 HTTP status code response.
	TimeDelay      time.Duration                  // TimeDelay allows programmer to impose an artificial delay before the handler responds.
}

// Do returns a crafted http.Response for every http.Request, depending on the
// MockConfig.
func (mockConfig *MockConfig) Do(request *http.Request) (*http.Response, error) {
	if mockConfig.TimeDelay > 0 {
		time.Sleep(mockConfig.TimeDelay)
	}

	var query string
	var results []string
	var err error

	if mockConfig.Callback == nil {
		results, err = mockConfig.Results, mockConfig.Err
	} else if query, err = url.QueryUnescape(request.URL.RawQuery); err == nil {
		results, err = mockConfig.Callback(query)
	}
	if err != nil {
		return nil, err
	}
	response := &http.Response{
		Body: ioutil.NopCloser(strings.NewReader(strings.Join(results, "\n"))),
	}
	if mockConfig.RangeException != "" {
		response.Header = http.Header{}
		response.Header.Add("RangeException", mockConfig.RangeException)
	}
	if mockConfig.StatusCode != 0 {
		response.StatusCode = mockConfig.StatusCode
	} else {
		response.StatusCode = http.StatusOK
	}
	response.Status = http.StatusText(response.StatusCode)
	return response, nil
}

// NewMockClient returns a Client that returns the same response for every query
// based on mockConfig.
func NewMockClient(mockConfig *MockConfig) *Client {
	// Even though MockConfig will never contact the below list of servers,
	// we need to give it at least one value.
	rrs, err := newRoundRobinStrings([]string{"dummy.example.com"})
	if err != nil {
		panic(err) // should not get here
	}

	return &Client{
		httpClient: mockConfig,
		servers:    rrs,
	}
}
