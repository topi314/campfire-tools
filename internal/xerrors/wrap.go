package xerrors

func Unwrap(err error) []error {
	u, ok := err.(interface {
		Unwrap() []error
	})
	if !ok {
		return []error{err}
	}
	return u.Unwrap()
}
