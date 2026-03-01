package pullrequest

import "fmt"

// PipelineStatus represents the CI/CD pipeline (status check rollup) state of a PR
type PipelineStatus int

const (
	// PipelineStatusUnknown means no pipeline data is available
	PipelineStatusUnknown PipelineStatus = iota
	// PipelineStatusRunning means one or more checks are pending/in progress
	PipelineStatusRunning
	// PipelineStatusSuccess means all checks passed
	PipelineStatusSuccess
	// PipelineStatusFailed means one or more checks failed
	PipelineStatusFailed
)

// String returns a string representation of the pipeline status
func (p PipelineStatus) String() string {
	switch p {
	case PipelineStatusRunning:
		return "running"
	case PipelineStatusSuccess:
		return "success"
	case PipelineStatusFailed:
		return "failed"
	default:
		return "unknown"
	}
}

// Emoji returns a display emoji for the pipeline status
func (p PipelineStatus) Emoji() string {
	switch p {
	case PipelineStatusRunning:
		return "🟡"
	case PipelineStatusSuccess:
		return "🟢"
	case PipelineStatusFailed:
		return "🔴"
	case PipelineStatusUnknown:
		return "❓"
	default:
		return "❓"
	}
}

// Label returns a human-readable label for the pipeline status (e.g. "Passed", "Failed")
func (p PipelineStatus) Label() string {
	switch p {
	case PipelineStatusRunning:
		return "Running"
	case PipelineStatusSuccess:
		return "Passed"
	case PipelineStatusFailed:
		return "Failed"
	default:
		return "Unknown"
	}
}

// MarshalText implements encoding.TextMarshaler so PipelineStatus serialises as
// a stable lowercase string in JSON and other text-based formats.
func (p PipelineStatus) MarshalText() ([]byte, error) {
	return []byte(p.String()), nil
}

// UnmarshalText implements encoding.TextUnmarshaler.
func (p *PipelineStatus) UnmarshalText(text []byte) error {
	switch string(text) {
	case "unknown":
		*p = PipelineStatusUnknown
	case "running":
		*p = PipelineStatusRunning
	case "success":
		*p = PipelineStatusSuccess
	case "failed":
		*p = PipelineStatusFailed
	default:
		return fmt.Errorf("unknown pipeline status %q", string(text))
	}
	return nil
}
