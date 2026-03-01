package pullrequest

import "fmt"

// ReviewState represents the state of a pull request review
type ReviewState int

const (
	// ReviewStateApproved indicates the reviewer approved the PR
	ReviewStateApproved ReviewState = iota
	// ReviewStateChangesRequested indicates the reviewer requested changes
	ReviewStateChangesRequested
	// ReviewStateCommented indicates the reviewer left a comment without approving or requesting changes
	ReviewStateCommented
	// ReviewStateDismissed indicates a previous review was dismissed
	ReviewStateDismissed
)

// String returns a string representation of the review state
func (rs ReviewState) String() string {
	switch rs {
	case ReviewStateApproved:
		return "approved"
	case ReviewStateChangesRequested:
		return "changes_requested"
	case ReviewStateCommented:
		return "commented"
	case ReviewStateDismissed:
		return "dismissed"
	default:
		return "unknown"
	}
}

// Emoji returns a display emoji for the review state
func (rs ReviewState) Emoji() string {
	switch rs {
	case ReviewStateApproved:
		return "✅"
	case ReviewStateChangesRequested:
		return "❌"
	case ReviewStateCommented:
		return "💬"
	case ReviewStateDismissed:
		return "🚫"
	default:
		return "?"
	}
}

// Label returns a human-readable label for the review state
func (rs ReviewState) Label() string {
	switch rs {
	case ReviewStateApproved:
		return "approved"
	case ReviewStateChangesRequested:
		return "requested changes"
	case ReviewStateCommented:
		return "commented"
	case ReviewStateDismissed:
		return "dismissed"
	default:
		return "unknown"
	}
}

// MarshalText implements encoding.TextMarshaler so ReviewState serialises as a
// stable lowercase string in JSON and other text-based formats.
func (rs ReviewState) MarshalText() ([]byte, error) {
	return []byte(rs.String()), nil
}

// UnmarshalText implements encoding.TextUnmarshaler.
func (rs *ReviewState) UnmarshalText(text []byte) error {
	switch string(text) {
	case "approved":
		*rs = ReviewStateApproved
	case "changes_requested":
		*rs = ReviewStateChangesRequested
	case "commented":
		*rs = ReviewStateCommented
	case "dismissed":
		*rs = ReviewStateDismissed
	default:
		return fmt.Errorf("unknown review state %q", string(text))
	}
	return nil
}
