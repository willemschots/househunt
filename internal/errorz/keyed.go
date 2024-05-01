package errorz

type Keyed struct {
	Key string
	Err error
}

func (k Keyed) Error() string {
	return k.Key + ": " + k.Err.Error()
}

func (k Keyed) Unwrap() error {
	return k.Err
}
