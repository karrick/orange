package orange

import (
	"bytes"
	"fmt"
	"strings"
	"testing"
)

type accreter []byte

func newAccreter() *accreter {
	var a accreter
	return &a
}

func (a *accreter) Printf(f string, v ...interface{}) {
	s := fmt.Sprintf(f, v...)
	// fmt.Fprintf(os.Stderr, "Printf: %s", s)
	*a = append(*a, s...)
}

func (a *accreter) Bytes() []byte { return *a }

func ensureLogged(tb testing.TB, a *accreter, contains string) {
	tb.Helper()
	if contains == "" {
		if got, want := a.Bytes(), []byte(nil); len(got) != 0 {
			tb.Errorf("GOT: %q; WANT: %v", got, want)
		}
	} else {
		if got, want := a.Bytes(), []byte(contains); !bytes.Contains(got, want) {
			tb.Errorf("GOT: %q; WANT: %q", got, want)
		}
	}
}

func ensureError(tb testing.TB, err error, contains string) {
	tb.Helper()
	if contains == "" {
		if err != nil {
			tb.Fatalf("GOT: %v; WANT: %v", err, contains)
		}
	} else {
		if err == nil || !strings.Contains(err.Error(), contains) {
			tb.Errorf("GOT: %v; WANT: %v", err, contains)
		}
	}
}

func ensurePanic(tb testing.TB, want string, callback func()) {
	tb.Helper()
	defer func() {
		r := recover()
		if r == nil {
			tb.Fatalf("GOT: %v; WANT: %v", r, want)
			return
		}
		if got := fmt.Sprintf("%v", r); got != want {
			tb.Fatalf("GOT: %v; WANT: %v", got, want)
		}
	}()
	callback()
}

func ensureStringSlicesMatch(tb testing.TB, actual, expected []string) {
	tb.Helper()
	if got, want := len(actual), len(expected); got != want {
		tb.Errorf("GOT: %v; WANT: %v", got, want)
	}
	la := len(actual)
	le := len(expected)
	for i := 0; i < la || i < le; i++ {
		if i < la {
			if i < le {
				if got, want := actual[i], expected[i]; got != want {
					tb.Errorf("GOT: %q; WANT: %q", got, want)
				}
			} else {
				tb.Errorf("GOT: %q (extra)", actual[i])
			}
		} else if i < le {
			tb.Errorf("WANT: %q (missing)", expected[i])
		}
	}
}
