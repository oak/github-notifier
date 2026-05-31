package config

import (
	"context"
	"path/filepath"
	"time"

	"github.com/fsnotify/fsnotify"
	"github.com/rs/zerolog/log"
)

const (
	// debounceDuration is the time to wait after a file event before reloading.
	// Editors often save files in multiple steps (write tmp → rename), so we
	// debounce to avoid reloading partial writes.
	debounceDuration = 500 * time.Millisecond
)

// WatchForValidConfig watches the config file at path for changes.
// When a valid config (with a non-empty GitHub token) is detected, it sends
// the config on the returned channel and stops watching.
// The caller should cancel the context to stop watching early.
func WatchForValidConfig(ctx context.Context, path string) <-chan *Config {
	ch := make(chan *Config, 1)

	go func() {
		defer close(ch)

		watcher, err := fsnotify.NewWatcher()
		if err != nil {
			log.Error().Err(err).Msg("Failed to create file watcher")
			return
		}
		defer watcher.Close()

		// Watch the directory containing the config file, not the file itself.
		// Many editors (vim, emacs) write to a temp file and rename, which
		// removes the original inode and breaks direct file watches.
		dir := filepath.Dir(path)
		if err := watcher.Add(dir); err != nil {
			log.Error().Err(err).Str("dir", dir).Msg("Failed to watch config directory")
			return
		}

		base := filepath.Base(path)
		var debounceTimer *time.Timer

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

				// Only react to writes/creates/renames of our config file
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

				// Reset debounce timer
				if debounceTimer != nil {
					debounceTimer.Stop()
				}
				debounceTimer = time.AfterFunc(debounceDuration, func() {
					cfg := LoadConfigWithPath(path)
					if cfg.IsValid() {
						log.Info().Msg("Valid configuration detected — starting application")
						select {
						case ch <- cfg:
						case <-ctx.Done():
						}
					} else {
						log.Info().Msg("Config file saved but GitHub token still not set")
					}
				})

			case err, ok := <-watcher.Errors:
				if !ok {
					return
				}
				log.Warn().Err(err).Msg("File watcher error")
			}
		}
	}()

	return ch
}
