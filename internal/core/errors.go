package core

import "fmt"

type ErrorCode string

const (
	ErrGeneric                     ErrorCode = "generic_error"
	ErrInvalidArguments            ErrorCode = "invalid_arguments"
	ErrNotConfigured               ErrorCode = "not_configured"
	ErrConflict                    ErrorCode = "conflict"
	ErrAuthFailed                  ErrorCode = "auth_failed"
	ErrRepositoryFailed            ErrorCode = "repository_failed"
	ErrFilesystemFailed            ErrorCode = "filesystem_failed"
	ErrUnsafeSecretPattern         ErrorCode = "unsafe_secret_pattern"
	ErrRepositoryNotFound          ErrorCode = "repository_not_found"
	ErrUnsupportedCapability       ErrorCode = "unsupported_capability"
	ErrRepositoryLocked            ErrorCode = "repository_locked"
	ErrRepositoryDirtyOutsideScope ErrorCode = "repository_dirty_outside_scope"
)

var ExitCodes = map[ErrorCode]int{
	ErrGeneric: 1, ErrInvalidArguments: 2, ErrNotConfigured: 3, ErrConflict: 4,
	ErrAuthFailed: 5, ErrRepositoryFailed: 6, ErrFilesystemFailed: 7,
	ErrUnsafeSecretPattern: 8, ErrRepositoryNotFound: 9, ErrUnsupportedCapability: 10,
	ErrRepositoryLocked: 11, ErrRepositoryDirtyOutsideScope: 12,
}

type Error struct {
	Code    ErrorCode      `json:"code"`
	Message string         `json:"message"`
	Details map[string]any `json:"details,omitempty"`
	Cause   error          `json:"-"`
}

func (e *Error) Error() string { return e.Message }
func (e *Error) Unwrap() error { return e.Cause }

func NewError(code ErrorCode, message string, details map[string]any) *Error {
	return &Error{Code: code, Message: message, Details: details}
}

func WrapError(code ErrorCode, message string, cause error, details map[string]any) *Error {
	return &Error{Code: code, Message: message, Cause: cause, Details: details}
}

func AsError(err error) *Error {
	if err == nil {
		return nil
	}
	if local, ok := err.(*Error); ok {
		return local
	}
	return WrapError(ErrGeneric, err.Error(), err, nil)
}

func Invalidf(format string, args ...any) *Error {
	return NewError(ErrInvalidArguments, fmt.Sprintf(format, args...), nil)
}
