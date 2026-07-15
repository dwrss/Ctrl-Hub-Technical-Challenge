package domain

import "errors"

var (
	ErrNotFound     = errors.New("not found")
	ErrInvalidInput = errors.New("invalid input")
	// ErrDanglingReference indicates a record was found but references
	// another record (e.g. a user) that could not be resolved. Unlike
	// ErrNotFound,this indicates a server-side data-integrity problem
	// and not a client error
	ErrDanglingReference = errors.New("dangling reference")
)
