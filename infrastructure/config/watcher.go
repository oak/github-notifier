package config

import (
	"context"
	"path/filepath"
	"time"

	"github.com/fsnotify/fsnotify"
	"github.com/oak3/github-notifier/domain/pullrequest"
	"github.com/rs/zerolog/log"
)

const (
	// debounceDuration is the time to wait after a file event before reloading.
	// Editors often save files in multiple steps (write tmp → rename), so we
	// debounce to avoid reloading partial writes.
	debounceDuration = 500 * time.Millisecond
)

// watchDebouncedFileChanges emits a signal whenever path is written/created/
// renamed, after a small debounce window.
func watchDebouncedFileChanges(ctx context.Context, path string) <-chan struct{} {
	signals := make(chan struct{}, 1)

	go func() {
		defer close(signals)

		watcher, err := fsnotify.NewWatcher()
		if err != nil {
			log.Error().Err(err).Msg("Failed to create file watcher")
			return
		}
		defer watcher.Close()

		// Watch the directory containing the file, not the file itself.
		dir := filepath.Dir(path)
		if err := watcher.Add(dir); err != nil {
			log.Error().Err(err).Str("dir", dir).Msg("Failed to watch config directory")
			return
		}

		base := filepath.Base(path)
		var debounceTimer *time.Timer
		var debounceC <-chan time.Time

		for {
			select {
			case <-ctx.Done():
				if debounceTimer != nil {
					debounceTimer.Stop()
				}
				return

			case event, ok := <-watcher.Events:
				if !ok {
					return
				}

				if filepath.Base(event.Name) != base {
					continue
				}

				isWrite := event.Has(fsnotify.Write) || event.Has(fsnotify.Create) || event.Has(fsnotify.Rename)
				if !isWrite {
					continue
				}

				log.Debug().
					Str("event", event.Op.String()).
					Str("file", event.Name).
					Msg("Config file changed, debouncing reload")

				if debounceTimer == nil {
					debounceTimer = time.NewTimer(debounceDuration)
					debounceC = debounceTimer.C
					continue
				}

				if !debounceTimer.Stop() {
					select {
					case <-debounceTimer.C:
					default:
					}
				}
				debounceTimer.Reset(debounceDuration)

			case <-debounceC:
				debounceC = nil
				if debounceTimer != nil {
					debounceC = debounceTimer.C
				}
				select {
				case signals <- struct{}{}:
				case <-ctx.Done():
					return
				}
			}
		}
	}()

	return signals
}

// WatchForValidConfig watches the config file at path for changes.
// When a valid config (with a non-empty GitHub token) is detected, it sends
// the config on the returned channel and stops watching.
// The caller should cancel the context to stop watching early.
func WatchForValidConfig(ctx context.Context, path string) <-chan *Config {
	ch := make(chan *Config, 1)

	go func() {
		defer close(ch)

		for range watchDebouncedFileChanges(ctx, path) {
			cfg := LoadConfigWithPath(path)
			if !cfg.IsValid() {
				log.Info().Msg("Config file saved but GitHub token still not set")
				continue
			}

			log.Info().Msg("Valid configuration detected — starting application")
			select {
			case ch <- cfg:
			case <-ctx.Done():
			}
			return
		}
	}()

	return ch
}

// WatchForValidIgnoreConfig watches ignoreFilePath and sends a freshly parsed
// *pullrequest.IgnoreConfig whenever the file changes and parses successfully.
// The channel is closed when ctx is cancelled.
func WatchForValidIgnoreConfig(ctx context.Context, ignoreFilePath string) <-chan *pullrequest.IgnoreConfig {
	ch := make(chan *pullrequest.IgnoreConfig, 1)

	go func() {
		defer close(ch)

		for range watchDebouncedFileChanges(ctx, ignoreFilePath) {
			cfg, err := LoadIgnoreConfig(ignoreFilePath)
			if err != nil || cfg == nil {
				continue
			}

			select {
			case ch <- cfg:
			case <-ctx.Done():
				return
			}
		}
	}()

	return ch
}
