package pullrequest

import "errors"

var (
	// ErrInvalidPRIdentifier indicates an invalid PR identifier
	ErrInvalidPRIdentifier = errors.New("invalid pull request identifier")

	// ErrInvalidRepository indicates an invalid repository
	ErrInvalidRepository = errors.New("invalid repository")

	// ErrInvalidAuthor indicates an invalid author
	ErrInvalidAuthor = errors.New("invalid author")

	// ErrPullRequestNotFound indicates a PR was not found
	ErrPullRequestNotFound = errors.New("pull request not found")

	// ErrRepositoryNotFound indicates a repository was not found
	ErrRepositoryNotFound = errors.New("repository not found")
)
