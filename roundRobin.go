package orange

import (
	"errors"
	"sync/atomic"
)

// roundRobinStrings returns a structure that on each invocation of its Next()
// method, returns the next string value from the list of values when it was
// initialized.  On rollover, it returns the first value from the list.
type roundRobinStrings struct {
	values []string
	i      uint32
}

func newRoundRobinStrings(someStrings []string) (*roundRobinStrings, error) {
	l := len(someStrings)
	if l == 0 {
		return nil, errors.New("cannot create a round robin strings structure without at least one string")
	}

	// Create the data structure
	rrs := &roundRobinStrings{
		values: make([]string, l),
	}

	// Populate data structure with values.
	copy(rrs.values, someStrings)

	return rrs, nil
}

// Len returns the number of strings in the roundRobinStrings structure.
func (rr *roundRobinStrings) Len() int { return len(rr.values) }

// Next returns the next string in the roundRobinStrings structure.
func (rr *roundRobinStrings) Next() string {
	l := uint32(len(rr.values))

	// Fast case when only a single value in list.
	if l == 1 {
		return rr.values[0]
	}

	var i, ni uint32

	for j := 0; j < 4; j++ {
		i = atomic.LoadUint32(&rr.i)
		ni = (i + 1) % l
		if atomic.CompareAndSwapUint32(&rr.i, i, ni) {
			return rr.values[i]
		}
	}

	// During high contention, give up and send back the string corresponding to
	// our last attempt.  This use-case does not require absolute perfect round
	// robin order.  Do not let perfect be the enemy of good enough.
	return rr.values[i]
}
