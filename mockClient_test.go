package orange

import (
	"context"
	"errors"
	"net/http"
	"testing"
	"time"
)

func TestNewMockClient(t *testing.T) {
	t.Run("Callback", func(t *testing.T) {
		var invoked bool
		client := NewMockClient(&MockConfig{
			Callback: func(query string) ([]string, error) {
				invoked = true
				if got, want := query, "q1"; got != want {
					t.Errorf("GOT: %v; WANT: %v", got, want)
				}
				return nil, nil
			},
		})
		_, err := client.Query("q1")
		ensureError(t, err)
		if got, want := invoked, true; got != want {
			t.Errorf("GOT: %v; WANT: %v", got, want)
		}
	})

	t.Run("Results", func(t *testing.T) {
		t.Run("empty", func(t *testing.T) {
			results := []string{}
			client := NewMockClient(&MockConfig{Results: results})

			results, err := client.Query("foo")

			ensureStringSlicesMatch(t, results, results)
			ensureError(t, err)

			t.Run("stats", func(t *testing.T) {
				stats := client.Stats()
				if got, want := stats.ErrContextDone, uint64(0); got != want {
					t.Errorf("GOT: %v; WANT: %v", got, want)
				}
				if got, want := stats.ErrRangeException, uint64(0); got != want {
					t.Errorf("GOT: %v; WANT: %v", got, want)
				}
				if got, want := stats.ErrStatusNotOK, uint64(0); got != want {
					t.Errorf("GOT: %v; WANT: %v", got, want)
				}
				if got, want := stats.ErrUnknown, uint64(0); got != want {
					t.Errorf("GOT: %v; WANT: %v", got, want)
				}
				if got, want := stats.NoErr, uint64(1); got != want {
					t.Errorf("GOT: %v; WANT: %v", got, want)
				}
			})
		})
		t.Run("single", func(t *testing.T) {
			results := []string{"alpha"}
			client := NewMockClient(&MockConfig{Results: results})

			results, err := client.Query("foo")

			ensureStringSlicesMatch(t, results, results)
			ensureError(t, err)

			t.Run("stats", func(t *testing.T) {
				stats := client.Stats()
				if got, want := stats.ErrContextDone, uint64(0); got != want {
					t.Errorf("GOT: %v; WANT: %v", got, want)
				}
				if got, want := stats.ErrRangeException, uint64(0); got != want {
					t.Errorf("GOT: %v; WANT: %v", got, want)
				}
				if got, want := stats.ErrStatusNotOK, uint64(0); got != want {
					t.Errorf("GOT: %v; WANT: %v", got, want)
				}
				if got, want := stats.ErrUnknown, uint64(0); got != want {
					t.Errorf("GOT: %v; WANT: %v", got, want)
				}
				if got, want := stats.NoErr, uint64(1); got != want {
					t.Errorf("GOT: %v; WANT: %v", got, want)
				}
			})
		})
		t.Run("double", func(t *testing.T) {
			results := []string{"alpha", "bravo"}
			client := NewMockClient(&MockConfig{Results: results})

			results, err := client.Query("foo")

			ensureStringSlicesMatch(t, results, results)
			ensureError(t, err)

			t.Run("stats", func(t *testing.T) {
				stats := client.Stats()
				if got, want := stats.ErrContextDone, uint64(0); got != want {
					t.Errorf("GOT: %v; WANT: %v", got, want)
				}
				if got, want := stats.ErrRangeException, uint64(0); got != want {
					t.Errorf("GOT: %v; WANT: %v", got, want)
				}
				if got, want := stats.ErrStatusNotOK, uint64(0); got != want {
					t.Errorf("GOT: %v; WANT: %v", got, want)
				}
				if got, want := stats.ErrUnknown, uint64(0); got != want {
					t.Errorf("GOT: %v; WANT: %v", got, want)
				}
				if got, want := stats.NoErr, uint64(1); got != want {
					t.Errorf("GOT: %v; WANT: %v", got, want)
				}
			})
		})
	})

	t.Run("Err", func(t *testing.T) {
		client := NewMockClient(&MockConfig{
			Err:     errors.New("flubber"),
			Results: []string{"alpha", "bravo"},
		})

		results, err := client.Query("foo")

		ensureStringSlicesMatch(t, results, nil) // no results
		ensureError(t, err, "flubber")

		t.Run("stats", func(t *testing.T) {
			stats := client.Stats()
			if got, want := stats.ErrContextDone, uint64(0); got != want {
				t.Errorf("GOT: %v; WANT: %v", got, want)
			}
			if got, want := stats.ErrRangeException, uint64(0); got != want {
				t.Errorf("GOT: %v; WANT: %v", got, want)
			}
			if got, want := stats.ErrStatusNotOK, uint64(0); got != want {
				t.Errorf("GOT: %v; WANT: %v", got, want)
			}
			if got, want := stats.ErrUnknown, uint64(1); got != want {
				t.Errorf("GOT: %v; WANT: %v", got, want)
			}
			if got, want := stats.NoErr, uint64(0); got != want {
				t.Errorf("GOT: %v; WANT: %v", got, want)
			}
		})
	})

	t.Run("RangeException", func(t *testing.T) {
		client := NewMockClient(&MockConfig{
			RangeException: "flubber",
			Results:        []string{"alpha", "bravo"},
		})

		results, err := client.Query("foo")

		ensureStringSlicesMatch(t, results, nil) // no results

		// ensure body read and provided
		switch tv := err.(type) {
		case ErrRangeException:
			ensureStringSlicesMatch(t, lines(tv.Body), []string{"alpha", "bravo"})
			ensureError(t, err, "flubber")
		default:
			t.Fatalf("GOT: %T; WANT: %T", err, ErrRangeException{})
		}

		t.Run("stats", func(t *testing.T) {
			stats := client.Stats()
			if got, want := stats.ErrContextDone, uint64(0); got != want {
				t.Errorf("GOT: %v; WANT: %v", got, want)
			}
			if got, want := stats.ErrRangeException, uint64(1); got != want {
				t.Errorf("GOT: %v; WANT: %v", got, want)
			}
			if got, want := stats.ErrStatusNotOK, uint64(0); got != want {
				t.Errorf("GOT: %v; WANT: %v", got, want)
			}
			if got, want := stats.ErrUnknown, uint64(0); got != want {
				t.Errorf("GOT: %v; WANT: %v", got, want)
			}
			if got, want := stats.NoErr, uint64(0); got != want {
				t.Errorf("GOT: %v; WANT: %v", got, want)
			}
		})
	})

	t.Run("StatusCode", func(t *testing.T) {
		client := NewMockClient(&MockConfig{
			Results:    []string{"alpha", "bravo"},
			StatusCode: http.StatusBadRequest,
		})

		results, err := client.Query("foo")

		ensureStringSlicesMatch(t, results, nil) // no results

		// ensure body read and provided
		switch tv := err.(type) {
		case ErrStatusNotOK:
			ensureStringSlicesMatch(t, lines(tv.Body), []string{"alpha", "bravo"})
			ensureError(t, err, http.StatusText(http.StatusBadRequest))
		default:
			t.Fatalf("GOT: %T; WANT: %T", err, ErrStatusNotOK{})
		}

		t.Run("stats", func(t *testing.T) {
			stats := client.Stats()
			if got, want := stats.ErrContextDone, uint64(0); got != want {
				t.Errorf("GOT: %v; WANT: %v", got, want)
			}
			if got, want := stats.ErrRangeException, uint64(0); got != want {
				t.Errorf("GOT: %v; WANT: %v", got, want)
			}
			if got, want := stats.ErrStatusNotOK, uint64(1); got != want {
				t.Errorf("GOT: %v; WANT: %v", got, want)
			}
			if got, want := stats.ErrUnknown, uint64(0); got != want {
				t.Errorf("GOT: %v; WANT: %v", got, want)
			}
			if got, want := stats.NoErr, uint64(0); got != want {
				t.Errorf("GOT: %v; WANT: %v", got, want)
			}
		})
	})

	t.Run("TimeDelay", func(t *testing.T) {
		const timeout = time.Millisecond

		client := NewMockClient(&MockConfig{
			Results:   []string{"alpha", "bravo"},
			TimeDelay: timeout << 1,
		})

		ctx, cancel := context.WithTimeout(context.Background(), timeout)
		defer cancel()

		results, err := client.QueryCtx(ctx, "foo")

		ensureStringSlicesMatch(t, results, nil) // no results
		ensureError(t, err, context.DeadlineExceeded.Error())

		t.Run("stats", func(t *testing.T) {
			stats := client.Stats()
			if got, want := stats.ErrContextDone, uint64(1); got != want {
				t.Errorf("GOT: %v; WANT: %v", got, want)
			}
			if got, want := stats.ErrRangeException, uint64(0); got != want {
				t.Errorf("GOT: %v; WANT: %v", got, want)
			}
			if got, want := stats.ErrStatusNotOK, uint64(0); got != want {
				t.Errorf("GOT: %v; WANT: %v", got, want)
			}
			if got, want := stats.ErrUnknown, uint64(0); got != want {
				t.Errorf("GOT: %v; WANT: %v", got, want)
			}
			if got, want := stats.NoErr, uint64(0); got != want {
				t.Errorf("GOT: %v; WANT: %v", got, want)
			}
		})
	})
}
