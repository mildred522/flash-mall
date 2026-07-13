package apperror

import "fmt"

// Error carries a stable business code and a user-facing message.
type Error struct {
	Code    Code
	Message string
	Cause   error
}

func New(code Code, message string) *Error {
	if message == "" {
		message = DefaultMessage(code)
	}
	return &Error{Code: code, Message: message}
}

func Wrap(code Code, message string, cause error) *Error {
	if message == "" {
		message = DefaultMessage(code)
	}
	return &Error{Code: code, Message: message, Cause: cause}
}

func (e *Error) Error() string {
	if e == nil {
		return ""
	}
	if e.Cause == nil {
		return fmt.Sprintf("%s: %s", e.Code, e.Message)
	}
	return fmt.Sprintf("%s: %s: %v", e.Code, e.Message, e.Cause)
}

func (e *Error) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.Cause
}
