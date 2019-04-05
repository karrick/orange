package orange

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func withServer(tb testing.TB, h func(w http.ResponseWriter, r *http.Request), callback func(*Client)) {
	server := httptest.NewServer(http.HandlerFunc(h))
	defer server.Close()

	client, err := NewClient(&Config{
		HTTPClient: server.Client(),
		Servers:    []string{strings.TrimLeft(server.URL, "http://")},
	})
	if err != nil {
		tb.Fatal(err)
	}

	callback(client)
}

func TestClient(t *testing.T) {
	t.Run("normal", func(t *testing.T) {
		t.Run("empty", func(t *testing.T) {
			h := func(w http.ResponseWriter, r *http.Request) {
				// does not write anything
			}
			withServer(t, h, func(client *Client) {
				response, err := client.Query("foo")
				if err != nil {
					t.Fatal(err)
				}
				ensureStringSlicesMatch(t, response.Split(), nil)
			})
		})

		t.Run("single", func(t *testing.T) {
			h := func(w http.ResponseWriter, r *http.Request) {
				if _, err := w.Write([]byte("result1")); err != nil {
					t.Fatal(err)
				}
			}
			withServer(t, h, func(client *Client) {
				response, err := client.Query("foo")
				if err != nil {
					t.Fatal(err)
				}
				ensureStringSlicesMatch(t, response.Split(), []string{"result1"})
			})
		})

		t.Run("double", func(t *testing.T) {
			h := func(w http.ResponseWriter, r *http.Request) {
				if _, err := w.Write([]byte("result1\nresult2")); err != nil {
					t.Fatal(err)
				}
			}
			withServer(t, h, func(client *Client) {
				response, err := client.Query("foo")
				if err != nil {
					t.Fatal(err)
				}
				ensureStringSlicesMatch(t, response.Split(), []string{"result1", "result2"})
			})
		})
	})

	t.Run("errors", func(t *testing.T) {
		t.Run("RangeException", func(t *testing.T) {
			h := func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("RangeException", "some error")
			}
			withServer(t, h, func(client *Client) {
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
			withServer(t, h, func(client *Client) {
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
}
