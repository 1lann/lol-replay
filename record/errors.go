package record

import (
	"strings"
)

// RecordingError is the error container most, if not all, errors from
// the methods in the record package will return.
type RecordingError struct {
	OpStack []string
	Err     error
}

// Error formats the error with its stack into a string.
func (r *RecordingError) Error() string {
	if len(r.OpStack) > 0 {
		return "record: " + strings.Join(r.OpStack, ": ") + ": " +
			r.Err.Error()
	}

	return "record: " + r.Err.Error()
}

func newError(op string, err error) error {
	if err == nil {
		panic("record: received nil error")
	}

	recErr, ok := err.(*RecordingError)
	if ok {
		if op == "" {
			return &RecordingError{nil, recErr.Err}
		}

		return &RecordingError{
			append([]string{op}, recErr.OpStack...),
			recErr.Err,
		}
	}

	if op == "" {
		return &RecordingError{nil, err}
	}

	return &RecordingError{[]string{op}, err}
}
