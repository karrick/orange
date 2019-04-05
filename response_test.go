package orange

import "testing"

func TestResponse(t *testing.T) {
	t.Run("empty", func(t *testing.T) {
		results := &Response{buf: []byte("\n")}
		ensureStringSlicesMatch(t, results.Split(), nil)
	})
	t.Run("single", func(t *testing.T) {
		results := &Response{buf: []byte("one\n")}
		ensureStringSlicesMatch(t, results.Split(), []string{"one"})
	})
	t.Run("double", func(t *testing.T) {
		results := &Response{buf: []byte("one\ntwo\n")}
		ensureStringSlicesMatch(t, results.Split(), []string{"one", "two"})
	})
	t.Run("triple", func(t *testing.T) {
		results := &Response{buf: []byte("one\ntwo\nthree\n")}
		ensureStringSlicesMatch(t, results.Split(), []string{"one", "two", "three"})
	})
}
