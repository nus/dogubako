package mtpfs

import (
	"errors"
	"fmt"
)

// retryExhaustedError is returned after a USB/session failure that was
// retried (or could not be retried) and still left the connection unusable.
type retryExhaustedError struct {
	err error
}

func (e retryExhaustedError) Error() string {
	if e.err == nil {
		return "MTP session failed"
	}
	return e.err.Error()
}

func (e retryExhaustedError) Unwrap() error { return e.err }

func retryExhausted(err error) error {
	if err == nil {
		err = fmt.Errorf("MTP session closed")
	}
	var re retryExhaustedError
	if errors.As(err, &re) {
		return err
	}
	return retryExhaustedError{err: err}
}

// IsRetryExhausted reports whether err is a failed MTP session that was
// already retried (or dropped) and needs a fresh connection.
func IsRetryExhausted(err error) bool {
	var re retryExhaustedError
	return errors.As(err, &re)
}

// RetryExhausted wraps err so IsRetryExhausted reports true. Tests use this
// to simulate a USB session that did not recover.
func RetryExhausted(err error) error {
	return retryExhausted(err)
}
