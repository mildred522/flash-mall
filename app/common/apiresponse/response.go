package apiresponse

import "flash-mall/app/common/apperror"

// Response is the shared JSON envelope for gateway and API handlers.
type Response struct {
	Code      apperror.Code `json:"code"`
	Message   string        `json:"message"`
	RequestID string        `json:"request_id,omitempty"`
	Data      any           `json:"data,omitempty"`
}

func OK(requestID string, data any) Response {
	return Response{
		Code:      apperror.CodeOK,
		Message:   apperror.DefaultMessage(apperror.CodeOK),
		RequestID: requestID,
		Data:      data,
	}
}

func Fail(requestID string, err error) Response {
	if err == nil {
		return OK(requestID, nil)
	}
	appErr := apperror.FromError(err)
	return Response{
		Code:      appErr.Code,
		Message:   appErr.Message,
		RequestID: requestID,
	}
}
