package pullrequest

import "errors"

var (
	// ErrInvalidPRIdentifier indicates an invalid PR identifier
	ErrInvalidPRIdentifier = errors.New("invalid pull request identifier")
)
