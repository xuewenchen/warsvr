package pkg

import "net/http"

// HTTPError is an error that carries an HTTP status code.
type HTTPError struct {
	Code int
	Msg  string
}

func (e *HTTPError) Error() string { return e.Msg }

// NewHTTPError creates an HTTPError with the given status code and message.
func NewHTTPError(code int, msg string) *HTTPError {
	return &HTTPError{Code: code, Msg: msg}
}

// ErrUnauthorized returns an HTTP 401 error with the given message.
func ErrUnauthorized(msg string) *HTTPError {
	return NewHTTPError(http.StatusUnauthorized, msg)
}
