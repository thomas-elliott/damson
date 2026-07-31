package axi

import (
	"errors"
	"fmt"
	"net/http"
)

// Code is the stable, machine-readable error label. An agent branches on this
// rather than parsing the message.
type Code string

const (
	CodeUsage       Code = "usage"
	CodeAuth        Code = "auth"
	CodeForbidden   Code = "forbidden"
	CodeNotFound    Code = "not_found"
	CodeConflict    Code = "conflict"
	CodeTooLarge    Code = "too_large"
	CodeRateLimited Code = "rate_limited"
	CodeServer      Code = "server"
	CodeNetwork     Code = "network"
	CodeNoSpace     Code = "no_space"
	CodeIO          Code = "io"
)

// Error is the structured failure every command returns. Help carries the
// concrete commands that would resolve it.
type Error struct {
	Code    Code
	Message string
	Help    []string
	// Err is the underlying cause, kept for errors.Is/As. It is never
	// rendered — the message is what the caller sees.
	Err error
}

func (e *Error) Error() string { return string(e.Code) + ": " + e.Message }

func (e *Error) Unwrap() error { return e.Err }

// ExitCode maps the error onto a process exit status: 2 when the command
// itself was malformed, 1 for everything else.
func (e *Error) ExitCode() int {
	if e.Code == CodeUsage {
		return ExitUsage
	}
	return ExitError
}

// Usagef builds a usage error, optionally carrying the commands that would
// have worked.
func Usagef(help []string, format string, args ...any) *Error {
	return &Error{Code: CodeUsage, Message: fmt.Sprintf(format, args...), Help: help}
}

// Errorf builds a non-usage error with the given code.
func Errorf(code Code, format string, args ...any) *Error {
	return &Error{Code: code, Message: fmt.Sprintf(format, args...)}
}

// Wrap attaches a code and message to an underlying cause.
func Wrap(code Code, err error, format string, args ...any) *Error {
	return &Error{Code: code, Message: fmt.Sprintf(format, args...), Err: err}
}

// AsError coerces any error into the envelope. An error that isn't already
// one is reported as a generic failure rather than losing its message.
func AsError(err error) *Error {
	if err == nil {
		return &Error{Code: CodeServer, Message: "unknown error"}
	}
	var e *Error
	if errors.As(err, &e) {
		return e
	}
	return &Error{Code: CodeIO, Message: err.Error(), Err: err}
}

// FromHTTP maps a server response onto a code. serverMsg is the API's
// {"error": …} body, which carries the actionable detail for 4xx; 5xx bodies
// are deliberately generic, so the status is all there is to report.
func FromHTTP(status int, serverMsg string) *Error {
	msg := serverMsg
	if msg == "" {
		msg = http.StatusText(status)
		if msg == "" {
			msg = fmt.Sprintf("HTTP %d", status)
		}
	}
	code := CodeServer
	switch {
	case status == http.StatusUnauthorized:
		code = CodeAuth
	case status == http.StatusForbidden:
		code = CodeForbidden
	case status == http.StatusNotFound:
		code = CodeNotFound
	case status == http.StatusConflict:
		code = CodeConflict
	case status == http.StatusRequestEntityTooLarge:
		code = CodeTooLarge
	case status == http.StatusTooManyRequests:
		code = CodeRateLimited
	case status == http.StatusBadRequest:
		code = CodeUsage
	case status >= 500:
		code = CodeServer
	}
	e := &Error{Code: code, Message: msg}
	switch code {
	case CodeAuth:
		e.Help = []string{"dmsn login --server <url> --token <token>", "dmsn whoami"}
	case CodeForbidden:
		e.Help = []string{"dmsn whoami"}
	}
	return e
}
