package orange

import (
	"bytes"
	"testing"
)

func TestResponse(t *testing.T) {
	t.Run("empty", func(t *testing.T) {
		r := &response{buf: nil}
		ensureStringSlicesMatch(t, r.Split(), nil)
	})
	t.Run("single", func(t *testing.T) {
		r := &response{buf: []byte("one")}
		ensureStringSlicesMatch(t, r.Split(), []string{"one"})
	})
	t.Run("double", func(t *testing.T) {
		r := &response{buf: []byte("one\ntwo")}
		ensureStringSlicesMatch(t, r.Split(), []string{"one", "two"})
	})
}

func TestNewResponse(t *testing.T) {
	run := func(tb testing.TB, input string, expected []string) {
		tb.Helper()
		r, err := newResponseFromReader(bytes.NewReader([]byte(input)))
		if err != nil {
			t.Fatal(err)
		}
		ensureStringSlicesMatch(tb, r.Split(), expected)
	}
	t.Run("empty", func(t *testing.T) {
		run(t, "", nil)
		run(t, "\n", nil)
	})
	t.Run("single", func(t *testing.T) {
		run(t, "one", []string{"one"})
		run(t, "one\n", []string{"one"})
	})
	t.Run("double", func(t *testing.T) {
		run(t, "one\ntwo", []string{"one", "two"})
		run(t, "one\ntwo\n", []string{"one", "two"})
	})
}
