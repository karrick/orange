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

func TestClient(t *testing.T) {
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
			}
			withClient(t, h, func(client *Client) {
				_, err := client.Query("foo")
				switch err.(type) {
				case ErrRangeException:
					ensureError(t, err, "some error")
				default:
					t.Errorf("GOT: %T; WANT: %T", err, ErrRangeException{})
				}
			})
		})

		t.Run("not ok", func(t *testing.T) {
			e := http.StatusBadGateway
			h := func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(e)
			}
			withClient(t, h, func(client *Client) {
				_, err := client.Query("foo")
				switch err.(type) {
				case ErrStatusNotOK:
					ensureError(t, err, http.StatusText(e))
				default:
					t.Errorf("GOT: %T; WANT: %T", err, ErrRangeException{})
				}
			})
		})
	})

	t.Run("Method retries", func(t *testing.T) {
		t.Run("GET fails", func(t *testing.T) {
			h := func(w http.ResponseWriter, r *http.Request) {
				if r.Method == http.MethodGet {
					http.Error(w, "too long", http.StatusRequestURITooLong)
					return
				}
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
		t.Run("PUT fails", func(t *testing.T) {
			h := func(w http.ResponseWriter, r *http.Request) {
				if r.Method == http.MethodPut {
					http.Error(w, "not allowed", http.StatusMethodNotAllowed)
					return
				}
				if _, err := w.Write([]byte("result1\nresult2\n")); err != nil {
					t.Fatal(err)
				}
			}
			withClient(t, h, func(client *Client) {
				// Force use of PUT by creating very long query
				var expression strings.Builder
				for i := 0; i < defaultQueryLengthThreshold; i++ {
					expression.WriteString(".")
				}

				values, err := client.Query(expression.String())
				if err != nil {
					t.Fatal(err)
				}
				ensureStringSlicesMatch(t, values, []string{"result1", "result2"})
			})
		})
	})
}
