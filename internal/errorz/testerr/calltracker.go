package testerr

// Calltracker is a helper struct to track calls to a dependency.
// It can be used to simulate failing dependencies.
// The zero value is ready to use and will never fail.
type Calltracker struct {
	CallIndex         int
	ShouldFail        bool
	Err               error
	FailAllAfterIndex bool
	FailAtIndex       int
}

// NewFailingDeps will create failing calltrackers that will fail
// at different points in the call sequence.
//
// Dependencies will fail in two ways:
// - A single failure, then all calls after succesful.
// - All calls will fail after a number of succesful calls.
func NewFailingDeps(err error, expectCalls int) []Calltracker {
	trackers := make([]Calltracker, 0, expectCalls*2)
	for i := 0; i < expectCalls; i++ {
		trackers = append(trackers, Calltracker{
			CallIndex:         -1,
			ShouldFail:        true,
			Err:               err,
			FailAllAfterIndex: true,
			FailAtIndex:       i,
		}, Calltracker{
			CallIndex:         -1,
			ShouldFail:        true,
			Err:               err,
			FailAllAfterIndex: false,
			FailAtIndex:       i,
		})
	}

	return trackers
}

// MaybeFailErrFunc fails the call if the next value in Fails is true.
func MaybeFailErrFunc(ct *Calltracker, f func() error) error {
	if ct.ShouldFail {
		ct.CallIndex++

		if ct.FailAtIndex == ct.CallIndex {
			return ct.Err
		}

		if ct.FailAllAfterIndex && ct.CallIndex > ct.FailAtIndex {
			return ct.Err
		}
	}

	return f()
}

// MaybeFail fails the call if the next value in Fails is true.
func MaybeFail[T any](ct *Calltracker, f func() (T, error)) (T, error) {
	if ct.ShouldFail {
		ct.CallIndex++

		var zero T

		if ct.FailAtIndex == ct.CallIndex {
			return zero, ct.Err
		}

		if ct.FailAllAfterIndex && ct.CallIndex > ct.FailAtIndex {
			return zero, ct.Err
		}
	}

	return f()
}
