package domain

import "chimera/platform/apperr"

type (
	Code  = apperr.Code
	Error = apperr.Error
)

const (
	CodeNotFound        = apperr.CodeNotFound
	CodeInvalid         = apperr.CodeInvalid
	CodeUnauthorized    = apperr.CodeUnauthorized
	CodeForbidden       = apperr.CodeForbidden
	CodeConflict        = apperr.CodeConflict
	CodeTooManyRequests = apperr.CodeTooManyRequests
	CodeUnavailable     = apperr.CodeUnavailable
	CodeInternal        = apperr.CodeInternal
)

var (
	NewError     = apperr.NewError
	Wrap         = apperr.Wrap
	NotFound     = apperr.NotFound
	Invalid      = apperr.Invalid
	Unauthorized = apperr.Unauthorized
	Forbidden    = apperr.Forbidden
	Conflict     = apperr.Conflict
	Unavailable  = apperr.Unavailable
	Internal     = apperr.Internal
	As           = apperr.As
	Is           = apperr.Is
)
