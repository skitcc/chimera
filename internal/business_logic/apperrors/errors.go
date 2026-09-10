package apperrors

import (
	"errors"
	"fmt"
)

type Code string

const (
	CodeNotFound     Code = "not_found"
	CodeInvalid      Code = "invalid"
	CodeUnauthorized Code = "unauthorized"
	CodeConflict     Code = "conflict"
	CodeInternal     Code = "internal"
)

type Error struct {
	Code    Code
	Message string
	err     error
}

func (e *Error) Error() string {
	if e.err != nil {
		return fmt.Sprintf("%s: %s: %v", e.Code, e.Message, e.err)
	}
	return fmt.Sprintf("%s: %s", e.Code, e.Message)
}

func (e *Error) Unwrap() error { return e.err }

func New(code Code, message string) *Error {
	return &Error{Code: code, Message: message}
}

func Wrap(code Code, message string, err error) *Error {
	return &Error{Code: code, Message: message, err: err}
}

func NotFound(message string) *Error     { return New(CodeNotFound, message) }
func Invalid(message string) *Error      { return New(CodeInvalid, message) }
func Unauthorized(message string) *Error { return New(CodeUnauthorized, message) }
func Conflict(message string) *Error     { return New(CodeConflict, message) }
func Internal(message string) *Error     { return New(CodeInternal, message) }

func As(err error) (*Error, bool) {
	var app *Error
	if errors.As(err, &app) {
		return app, true
	}
	return nil, false
}

func Is(err error, code Code) bool {
	app, ok := As(err)
	return ok && app.Code == code
}
