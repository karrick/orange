package orange

import "testing"

func TestRoundRobin(t *testing.T) {
	t.Run("empty", func(t *testing.T) {
		_, err := newRoundRobinStrings(nil)
		ensureError(t, err, "cannot create")
	})

	t.Run("single", func(t *testing.T) {
		rrs, err := newRoundRobinStrings([]string{"one"})
		ensureError(t, err)

		if got, want := rrs.Next(), "one"; got != want {
			t.Errorf("GOT: %v; WANT: %v", got, want)
		}

		if got, want := rrs.Next(), "one"; got != want {
			t.Errorf("GOT: %v; WANT: %v", got, want)
		}
	})

	t.Run("double", func(t *testing.T) {
		rrs, err := newRoundRobinStrings([]string{"one", "two"})
		ensureError(t, err)

		if got, want := rrs.Next(), "one"; got != want {
			t.Errorf("GOT: %v; WANT: %v", got, want)
		}

		if got, want := rrs.Next(), "two"; got != want {
			t.Errorf("GOT: %v; WANT: %v", got, want)
		}

		if got, want := rrs.Next(), "one"; got != want {
			t.Errorf("GOT: %v; WANT: %v", got, want)
		}
	})

	t.Run("triple", func(t *testing.T) {
		rrs, err := newRoundRobinStrings([]string{"one", "two", "three"})
		ensureError(t, err)

		if got, want := rrs.Next(), "one"; got != want {
			t.Errorf("GOT: %v; WANT: %v", got, want)
		}

		if got, want := rrs.Next(), "two"; got != want {
			t.Errorf("GOT: %v; WANT: %v", got, want)
		}

		if got, want := rrs.Next(), "three"; got != want {
			t.Errorf("GOT: %v; WANT: %v", got, want)
		}

		if got, want := rrs.Next(), "one"; got != want {
			t.Errorf("GOT: %v; WANT: %v", got, want)
		}
	})
}
