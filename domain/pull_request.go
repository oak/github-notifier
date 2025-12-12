package domain

import "time"

type PullRequest struct {
	Title     string
	URL       string
	Number    int
	CreatedAt time.Time
	Repository struct {
		NameWithOwner string
	}
	Author struct {
		Login string
	}
}
