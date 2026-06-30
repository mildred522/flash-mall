package apperror

import "errors"

func FromError(err error) *Error {
	if err == nil {
		return nil
	}
	var appErr *Error
	if errors.As(err, &appErr) {
		return appErr
	}
	return Wrap(CodeInternal, "", err)
}

func CodeOf(err error) Code {
	if err == nil {
		return CodeOK
	}
	return FromError(err).Code
}

func MessageOf(err error) string {
	if err == nil {
		return DefaultMessage(CodeOK)
	}
	return FromError(err).Message
}
