package apperr

import "errors"

type Kind int

const (
	KindValidation Kind = iota
	KindUnauthorized
	KindForbidden
	KindNotFound
	KindConflict
	KindRateLimited
	KindInternal
)

type Error struct {
	Kind   Kind
	Code   string
	Detail string
	Err    error
}

func (e *Error) Error() string {
	if e.Err != nil {
		return e.Code + ": " + e.Err.Error()
	}
	return e.Code
}

func (e *Error) Unwrap() error {
	return e.Err
}

func New(kind Kind, code, detail string) *Error {
	return &Error{Kind: kind, Code: code, Detail: detail}
}

func Wrap(kind Kind, code, detail string, err error) *Error {
	return &Error{Kind: kind, Code: code, Detail: detail, Err: err}
}

func IsKind(err error, kind Kind) bool {
	var target *Error
	if errors.As(err, &target) {
		return target.Kind == kind
	}
	return false
}

func Code(err error) string {
	var target *Error
	if errors.As(err, &target) {
		return target.Code
	}
	return ""
}

func Detail(err error) string {
	var target *Error
	if errors.As(err, &target) {
		return target.Detail
	}
	return ""
}

func Invalid(detail string) *Error {
	return New(KindValidation, "invalid_request", detail)
}

func Unauthorized(code string) *Error {
	return New(KindUnauthorized, code, "Authentication is required.")
}

func Forbidden(code string) *Error {
	return New(KindForbidden, code, "You do not have permission to perform this action.")
}

func NotFound(code string) *Error {
	return New(KindNotFound, code, "The requested resource was not found.")
}

func Conflict(code string) *Error {
	return New(KindConflict, code, "The resource already exists.")
}
