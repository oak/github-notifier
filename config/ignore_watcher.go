package config

import (
	"context"
	"os"
	"time"

	"github.com/oak3/github-notifier/domain/pullrequest"
)

// WatchForValidIgnoreConfig polls ignoreFilePath every 2 seconds and sends a
// freshly parsed *pullrequest.IgnoreConfig on the returned channel whenever the
// file's modification time changes and the new content parses successfully.
// The channel is closed when ctx is cancelled.
func WatchForValidIgnoreConfig(ctx context.Context, ignoreFilePath string) <-chan *pullrequest.IgnoreConfig {
	ch := make(chan *pullrequest.IgnoreConfig, 1)
	go func() {
		defer close(ch)
		var lastModTime time.Time
		for {
			select {
			case <-ctx.Done():
				return
			case <-time.After(2 * time.Second):
				info, err := os.Stat(ignoreFilePath)
				if err != nil || !info.ModTime().After(lastModTime) {
					continue
				}
				cfg, err := LoadIgnoreConfig(ignoreFilePath)
				if err != nil || cfg == nil {
					continue
				}
				lastModTime = info.ModTime()
				select {
				case ch <- cfg:
				case <-ctx.Done():
					return
				}
			}
		}
	}()
	return ch
}
