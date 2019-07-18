package orange

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func withTestServer(tb testing.TB, h func(w http.ResponseWriter, r *http.Request), callback func(*httptest.Server)) {
	server := httptest.NewServer(http.HandlerFunc(h))
	defer server.Close()
	callback(server)
}

func withClient(tb testing.TB, h func(w http.ResponseWriter, r *http.Request), callback func(*Client)) {
	withTestServer(tb, h, func(server *httptest.Server) {
		client, err := NewClient(&Config{
			HTTPClient: server.Client(),
			RetryCount: 2,
			Servers:    []string{strings.TrimLeft(server.URL, "http://")},
		})
		if err != nil {
			tb.Fatal(err)
		}
		callback(client)
	})
}

func lines(buf []byte) []string {
	s := string(buf)
	s = strings.TrimRight(s, "\n")
	s = strings.Replace(s, "\r", "", -1)
	return strings.Split(s, "\n")
}

func TestClient(t *testing.T) {
	// NOTE: Following tests invoke the Query method, indirectly also testing
	// the QueryCtx and QueryCallback methods.

	t.Run("correct request format", func(t *testing.T) {
		t.Run("GET", func(t *testing.T) {
			h := func(w http.ResponseWriter, r *http.Request) {
				if got, want := r.URL.Path, "/range/list"; got != want {
					t.Errorf("GOT: %v; WANT: %v", got, want)
				}
				if got, want := r.URL.RawQuery, "%7Bfoo%2Cbar%7D"; got != want {
					t.Errorf("GOT: %v; WANT: %v", got, want)
				}
			}
			withClient(t, h, func(client *Client) {
				_, err := client.Query("{foo,bar}")
				if err != nil {
					t.Fatal(err)
				}
			})
		})
		t.Run("PUT", func(t *testing.T) {
			// Force initial use of PUT by creating very long query.
			var expression, requestBody strings.Builder
			requestBody.WriteString("query=")
			for i := 0; i < defaultQueryURILengthThreshold; i++ {
				expression.WriteString("{")
				requestBody.WriteString("%7B")
			}

			h := func(w http.ResponseWriter, r *http.Request) {
				if got, want := r.URL.Path, "/range/list"; got != want {
					t.Errorf("GOT: %v; WANT: %v", got, want)
				}
				buf, err := bytesFromReadCloser(r.Body)
				if err != nil {
					t.Fatal(err)
				}
				if got, want := string(buf), requestBody.String(); got != want {
					t.Errorf("GOT: %v; WANT: %v", got, want)
				}
			}
			withClient(t, h, func(client *Client) {
				_, err := client.Query(expression.String())
				if err != nil {
					t.Fatal(err)
				}
			})
		})
	})
	t.Run("normal", func(t *testing.T) {
		t.Run("empty", func(t *testing.T) {
			t.Run("sans newline", func(t *testing.T) {
				h := func(w http.ResponseWriter, r *http.Request) {
					// does not write anything
				}
				withClient(t, h, func(client *Client) {
					values, err := client.Query("foo")
					if err != nil {
						t.Fatal(err)
					}
					ensureStringSlicesMatch(t, values, nil)
				})
			})
			t.Run("with newline", func(t *testing.T) {
				h := func(w http.ResponseWriter, r *http.Request) {
					if _, err := w.Write([]byte{'\n'}); err != nil {
						t.Fatal(err)
					}
				}
				withClient(t, h, func(client *Client) {
					values, err := client.Query("foo")
					if err != nil {
						t.Fatal(err)
					}
					ensureStringSlicesMatch(t, values, nil)
				})
			})
		})

		t.Run("single", func(t *testing.T) {
			t.Run("sans newline", func(t *testing.T) {
				h := func(w http.ResponseWriter, r *http.Request) {
					if _, err := w.Write([]byte("result1")); err != nil {
						t.Fatal(err)
					}
				}
				withClient(t, h, func(client *Client) {
					values, err := client.Query("foo")
					if err != nil {
						t.Fatal(err)
					}
					ensureStringSlicesMatch(t, values, []string{"result1"})
				})
			})

			t.Run("with newline", func(t *testing.T) {
				h := func(w http.ResponseWriter, r *http.Request) {
					if _, err := w.Write([]byte("result1\n")); err != nil {
						t.Fatal(err)
					}
				}
				withClient(t, h, func(client *Client) {
					values, err := client.Query("foo")
					if err != nil {
						t.Fatal(err)
					}
					ensureStringSlicesMatch(t, values, []string{"result1"})
				})
			})
		})

		t.Run("double", func(t *testing.T) {
			t.Run("sans newline", func(t *testing.T) {
				h := func(w http.ResponseWriter, r *http.Request) {
					if _, err := w.Write([]byte("result1\nresult2")); err != nil {
						t.Fatal(err)
					}
				}
				withClient(t, h, func(client *Client) {
					values, err := client.Query("foo")
					if err != nil {
						t.Fatal(err)
					}
					ensureStringSlicesMatch(t, values, []string{"result1", "result2"})
				})
			})

			t.Run("with newline", func(t *testing.T) {
				h := func(w http.ResponseWriter, r *http.Request) {
					if _, err := w.Write([]byte("result1\nresult2\n")); err != nil {
						t.Fatal(err)
					}
				}
				withClient(t, h, func(client *Client) {
					values, err := client.Query("foo")
					if err != nil {
						t.Fatal(err)
					}
					ensureStringSlicesMatch(t, values, []string{"result1", "result2"})
				})
			})
		})
	})

	t.Run("errors", func(t *testing.T) {
		t.Run("context times out", func(t *testing.T) {
			const timeout = time.Millisecond
			h := func(w http.ResponseWriter, r *http.Request) {
				time.Sleep(timeout << 2)
			}
			withClient(t, h, func(client *Client) {
				ctx, done := context.WithTimeout(context.Background(), timeout)
				defer done()
				_, err := client.QueryCtx(ctx, "foo")
				ensureError(t, err, "deadline")
			})
		})

		t.Run("RangeException", func(t *testing.T) {
			h := func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("RangeException", "some error")
				w.Write([]byte("body1\nbody2\n"))
			}
			withClient(t, h, func(client *Client) {
				response, err := client.Query("foo")
				switch err.(type) {
				case ErrRangeException:
					ensureError(t, err, "some error")
					if got, avoid := err.Error(), "body"; strings.Contains(got, avoid) {
						t.Errorf("GOT: %v; AVOID: %v", got, avoid)
					}
				default:
					t.Errorf("GOT: %T; WANT: %T", err, ErrRangeException{})
				}
				ensureStringSlicesMatch(t, response, nil)
			})
		})

		t.Run("RangeException carriage returns", func(t *testing.T) {
			h := func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("RangeException", "some error")
				w.Write([]byte("body1\r\nbody2\r\n"))
			}
			withClient(t, h, func(client *Client) {
				response, err := client.Query("foo")
				switch err.(type) {
				case ErrRangeException:
					ensureError(t, err, "some error")
					if got, avoid := err.Error(), "body"; strings.Contains(got, avoid) {
						t.Errorf("GOT: %v; AVOID: %v", got, avoid)
					}
				default:
					t.Errorf("GOT: %T; WANT: %T", err, ErrRangeException{})
				}
				ensureStringSlicesMatch(t, response, nil)
			})
		})

		t.Run("not ok", func(t *testing.T) {
			e := http.StatusBadGateway
			h := func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(e)
				w.Write([]byte("body1\nbody2\n"))
			}
			withClient(t, h, func(client *Client) {
				response, err := client.Query("foo")
				switch v := err.(type) {
				case ErrStatusNotOK:
					ensureError(t, err, http.StatusText(e))
					if got, avoid := err.Error(), "body"; strings.Contains(got, avoid) {
						t.Errorf("GOT: %v; AVOID: %v", got, avoid)
					}
					ensureStringSlicesMatch(t, lines(v.Body), []string{"body1", "body2"})
				default:
					t.Errorf("GOT: %T; WANT: %T", err, ErrRangeException{})
				}
				ensureStringSlicesMatch(t, response, nil)
			})
		})

		t.Run("not ok with carriage returns", func(t *testing.T) {
			e := http.StatusBadGateway
			h := func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(e)
				w.Write([]byte("body1\r\nbody2\r\n"))
			}
			withClient(t, h, func(client *Client) {
				response, err := client.Query("foo")
				switch v := err.(type) {
				case ErrStatusNotOK:
					ensureError(t, err, http.StatusText(e))
					if got, avoid := err.Error(), "body"; strings.Contains(got, avoid) {
						t.Errorf("GOT: %v; AVOID: %v", got, avoid)
					}
					ensureStringSlicesMatch(t, lines(v.Body), []string{"body1", "body2"})
				default:
					t.Errorf("GOT: %T; WANT: %T", err, ErrRangeException{})
				}
				ensureStringSlicesMatch(t, response, nil)
			})
		})
	})

	t.Run("retries query", func(t *testing.T) {
		t.Run("with PUT when server returns uri too long", func(t *testing.T) {
			var getInvocationCount, putInvocationCount int

			h := func(w http.ResponseWriter, r *http.Request) {
				switch r.Method {
				case http.MethodGet:
					getInvocationCount++
					http.Error(w, r.RequestURI, http.StatusRequestURITooLong)
				case http.MethodPut:
					putInvocationCount++
					if _, err := w.Write([]byte("result1\nresult2\n")); err != nil {
						t.Fatal(err)
					}
				default:
					http.Error(w, r.Method, http.StatusMethodNotAllowed)
				}
			}
			withClient(t, h, func(client *Client) {
				values, err := client.Query("foo")
				if err != nil {
					t.Fatal(err)
				}
				ensureStringSlicesMatch(t, values, []string{"result1", "result2"})
			})

			if got, want := getInvocationCount, 1; got != want {
				t.Errorf("GOT: %v; WANT: %v", got, want)
			}

			if got, want := putInvocationCount, 1; got != want {
				t.Errorf("GOT: %v; WANT: %v", got, want)
			}
		})
		t.Run("with GET when server returns method not allowed", func(t *testing.T) {
			var getInvocationCount, putInvocationCount int

			h := func(w http.ResponseWriter, r *http.Request) {
				switch r.Method {
				case http.MethodGet:
					getInvocationCount++
					if _, err := w.Write([]byte("result1\nresult2\n")); err != nil {
						t.Fatal(err)
					}
				case http.MethodPut:
					putInvocationCount++
					fallthrough
				default:
					http.Error(w, r.Method, http.StatusMethodNotAllowed)
				}
			}

			withClient(t, h, func(client *Client) {
				// Force initial use of PUT by creating very long query.
				var expression strings.Builder
				for i := 0; i < defaultQueryURILengthThreshold; i++ {
					expression.WriteString(".")
				}

				values, err := client.Query(expression.String())
				if err != nil {
					t.Fatal(err)
				}
				ensureStringSlicesMatch(t, values, []string{"result1", "result2"})
			})

			if got, want := getInvocationCount, 1; got != want {
				t.Errorf("GOT: %v; WANT: %v", got, want)
			}

			if got, want := putInvocationCount, 1; got != want {
				t.Errorf("GOT: %v; WANT: %v", got, want)
			}
		})
		t.Run("will not try GET multiple times", func(t *testing.T) {
			var getInvocationCount, putInvocationCount int

			h := func(w http.ResponseWriter, r *http.Request) {
				switch r.Method {
				case http.MethodGet:
					getInvocationCount++
				case http.MethodPut:
					putInvocationCount++
				}
				http.Error(w, "body1\nbody2\n", http.StatusServiceUnavailable)
			}

			withClient(t, h, func(client *Client) {
				_, err := client.Query("%some.short.expression")
				ensureError(t, err, http.StatusText(http.StatusServiceUnavailable))
				switch e := err.(type) {
				case ErrStatusNotOK:
					ensureStringSlicesMatch(t, lines(e.Body), []string{"body1", "body2"})
				default:
					t.Errorf("GOT: %v; WANT: %v", err, ErrStatusNotOK{})
				}
			})

			if got, want := getInvocationCount, 1; got != want {
				t.Errorf("GOT: %v; WANT: %v", got, want)
			}

			if got, want := putInvocationCount, 0; got != want {
				t.Errorf("GOT: %v; WANT: %v", got, want)
			}
		})
		t.Run("will not try PUT multiple times", func(t *testing.T) {
			var getInvocationCount, putInvocationCount int

			h := func(w http.ResponseWriter, r *http.Request) {
				switch r.Method {
				case http.MethodGet:
					getInvocationCount++
				case http.MethodPut:
					putInvocationCount++
				}
				http.Error(w, "body1\nbody2\n", http.StatusServiceUnavailable)
			}

			withClient(t, h, func(client *Client) {
				// Force initial use of PUT by creating very long query.
				var expression strings.Builder
				for i := 0; i < defaultQueryURILengthThreshold; i++ {
					expression.WriteString(".")
				}

				_, err := client.Query(expression.String())
				ensureError(t, err, http.StatusText(http.StatusServiceUnavailable))
				switch e := err.(type) {
				case ErrStatusNotOK:
					ensureStringSlicesMatch(t, lines(e.Body), []string{"body1", "body2"})
				default:
					t.Errorf("GOT: %v; WANT: %v", err, ErrStatusNotOK{})
				}
			})

			if got, want := getInvocationCount, 0; got != want {
				t.Errorf("GOT: %v; WANT: %v", got, want)
			}

			if got, want := putInvocationCount, 1; got != want {
				t.Errorf("GOT: %v; WANT: %v", got, want)
			}
		})
	})
}
