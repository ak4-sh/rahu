package jsonrpc

import (
	j "encoding/json"
	"fmt"
)

// ErrorCode is a JSON-RPC 2.0 error code.
type ErrorCode int

const (
	CodeParseError     ErrorCode = -32700
	CodeInvalidReq     ErrorCode = -32600
	CodeMethodNotFound ErrorCode = -32601
	CodeInvalidParams  ErrorCode = -32602
	CodeInternalError  ErrorCode = -32603
)

type Error struct {
	Code    ErrorCode    `json:"code"`
	Message string       `json:"message"`
	Data    j.RawMessage `json:"data,omitempty"`
}

func (e *Error) Error() string {
	return fmt.Sprintf("%d %s", e.Code, e.Message)
}

func NewError(code ErrorCode, msg string, data ...j.RawMessage) *Error {
	var d j.RawMessage
	if len(data) > 0 {
		d = data[0]
	}

	return &Error{
		Code:    code,
		Message: msg,
		Data:    d,
	}
}

func ParseError() *Error {
	return NewError(CodeParseError, "Parse error")
}

func InvalidRequestError() *Error {
	return NewError(CodeInvalidReq, "Invalid request")
}

func MethodNotFoundError() *Error {
	return NewError(CodeMethodNotFound, "Method not found")
}

func InvalidParamsError(d j.RawMessage) *Error {
	return NewError(CodeInvalidParams, "Invalid params", d)
}

func InternalError() *Error {
	return NewError(CodeInternalError, "Internal error")
}
