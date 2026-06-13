//go:build darwin

package un

/*
#cgo CFLAGS: -x objective-c
#cgo LDFLAGS: -framework Foundation -framework AppKit -framework UserNotifications
#include <stdlib.h>

int  ghn_available(void);
void ghn_setup(void);
void ghn_send(const char *identifier, const char *title, const char *body, const char *openURL);
*/
import "C"
import (
	"fmt"
	"strings"
	"sync"
	"unsafe"

	"github.com/rs/zerolog/log"

	"github.com/oak/github-notifier/application/port"
	"github.com/oak/github-notifier/domain/pullrequest"
)

var setupOnce sync.Once

// Adapter implements port.NotificationPort using UNUserNotificationCenter via cgo.
// It requires no external binaries and supports click-to-open on all macOS versions.
type Adapter struct{}

// NewAdapter creates and initialises the UNUserNotificationCenter adapter.
// Returns nil if the binary is not running inside a proper .app bundle,
// in which case the caller should fall back to another notification method.
func NewAdapter() *Adapter {
	if C.ghn_available() == 0 {
		return nil
	}
	setupOnce.Do(func() {
		C.ghn_setup()
	})
	return &Adapter{}
}

// NotifyPullRequests sends one notification per PR.
func (a *Adapter) NotifyPullRequests(notifications []*port.PRNotificationData) error {
	for _, n := range notifications {
		if err := a.sendPR(n); err != nil {
			log.Error().Err(err).Msg("UN: failed to send PR notification")
		}
	}
	return nil
}

// NotifyMessage sends a plain title+message notification.
func (a *Adapter) NotifyMessage(title, message string) error {
	a.send(title, title, message, "")
	return nil
}

// SupportsClickActions always returns true — click-to-open is handled natively.
func (a *Adapter) SupportsClickActions() bool { return true }

// ---- helpers ----------------------------------------------------------------

func (a *Adapter) sendPR(n *port.PRNotificationData) error {
	pr := n.PullRequest
	title := buildTitle(n)
	body := buildBody(n)
	id := fmt.Sprintf("pr-%d", pr.Number())
	a.send(id, title, body, pr.URL())
	return nil
}

func (a *Adapter) send(id, title, body, openURL string) {
	cID := C.CString(id)
	cTitle := C.CString(title)
	cBody := C.CString(body)
	cURL := C.CString(openURL)
	defer func() {
		C.free(unsafe.Pointer(cID))
		C.free(unsafe.Pointer(cTitle))
		C.free(unsafe.Pointer(cBody))
		C.free(unsafe.Pointer(cURL))
	}()
	C.ghn_send(cID, cTitle, cBody, cURL)
}

func buildTitle(n *port.PRNotificationData) string {
	pr := n.PullRequest
	if n.IsNew {
		return fmt.Sprintf("New PR #%d", pr.Number())
	}
	if n.PipelineChange != nil {
		s := n.PipelineChange.NewStatus
		return fmt.Sprintf("PR #%d Pipeline %s %s", pr.Number(), s.Label(), s.Emoji())
	}
	if len(n.ReviewChanges) > 0 {
		return fmt.Sprintf("PR #%d Review", pr.Number())
	}
	return fmt.Sprintf("PR #%d Activity", pr.Number())
}

func buildBody(n *port.PRNotificationData) string {
	pr := n.PullRequest
	var parts []string

	parts = append(parts, pr.RepositoryName())
	parts = append(parts, pr.Title())

	if n.IsNew {
		parts = append(parts, "New — needs review")
	}

	if n.PipelineChange != nil {
		s := n.PipelineChange.NewStatus
		parts = append(parts, fmt.Sprintf("Pipeline: %s %s", s.Emoji(), s.Label()))
	}

	activityOrder := []pullrequest.ActivityType{
		pullrequest.ActivityTypePush,
		pullrequest.ActivityTypeReview,
		pullrequest.ActivityTypeComment,
		pullrequest.ActivityTypeReaction,
		pullrequest.ActivityTypeCommit,
	}
	for _, actType := range activityOrder {
		for _, act := range n.Activities {
			if act.Type == actType {
				parts = append(parts, activityLabel(act.Type, act.Count))
			}
		}
	}

	for _, sc := range n.StatusChanges {
		switch sc.EventType {
		case pullrequest.StatusChangeMerged:
			parts = append(parts, "Merged")
		case pullrequest.StatusChangeClosed:
			parts = append(parts, "Closed")
		}
	}

	for _, rc := range n.ReviewChanges {
		parts = append(parts, fmt.Sprintf("%s %s %s", rc.State.Emoji(), rc.Reviewer, rc.State.Label()))
	}

	return strings.Join(parts, "\n")
}

func activityLabel(t pullrequest.ActivityType, count int) string {
	plural := func(n int, singular, plural string) string {
		if n == 1 {
			return fmt.Sprintf("1 %s", singular)
		}
		return fmt.Sprintf("%d %s", n, plural)
	}
	switch t {
	case pullrequest.ActivityTypePush:
		return plural(count, "new commit", "new commits")
	case pullrequest.ActivityTypeReview:
		return plural(count, "new review", "new reviews")
	case pullrequest.ActivityTypeComment:
		return plural(count, "new comment", "new comments")
	case pullrequest.ActivityTypeReaction:
		return plural(count, "new reaction", "new reactions")
	case pullrequest.ActivityTypeCommit:
		return plural(count, "new commit", "new commits")
	default:
		return plural(count, "new activity", "new activities")
	}
}
