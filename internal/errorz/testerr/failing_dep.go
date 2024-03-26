package testerr

type FailingDep struct {
	CallIndex         int
	Err               error
	FailAllAfterIndex bool
	FailAtIndex       int
}

// NewFailingDeps will create failure cases for a number of calls to a dependency.
//
// Dependencies will fail in two ways:
// - A single failure, then all calls after succesful.
// - All calls will fail after a number of succesful calls.
func NewFailingDeps(err error, expectCalls int) []FailingDep {
	deps := make([]FailingDep, 0, expectCalls*2)
	for i := 0; i < expectCalls; i++ {
		deps = append(deps, FailingDep{
			CallIndex:         -1,
			Err:               err,
			FailAllAfterIndex: true,
			FailAtIndex:       i,
		}, FailingDep{
			CallIndex:         -1,
			Err:               err,
			FailAllAfterIndex: false,
			FailAtIndex:       i,
		})
	}

	return deps
}

// MaybeFailErrFunc fails the call if the next value in Fails is true.
func MaybeFailErrFunc(dep *FailingDep, f func() error) error {
	dep.CallIndex++

	if dep.FailAtIndex == dep.CallIndex {
		return dep.Err
	}

	if dep.FailAllAfterIndex && dep.CallIndex > dep.FailAtIndex {
		return dep.Err
	}

	return f()
}

// MaybeFail fails the call if the next value in Fails is true.
func MaybeFail[T any](dep *FailingDep, f func() (T, error)) (T, error) {
	dep.CallIndex++

	var zero T

	if dep.FailAtIndex == dep.CallIndex {
		return zero, dep.Err
	}

	if dep.FailAllAfterIndex && dep.CallIndex > dep.FailAtIndex {
		return zero, dep.Err
	}

	return f()
}
