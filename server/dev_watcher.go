package server

import (
	"context"
	"crypto/sha1"
	"encoding/hex"
	"fmt"
	"io/fs"
	"log/slog"
	"path/filepath"
	"time"
)

// devWatcherInterval controls how frequently the dev watcher checks for changes.
const devWatcherInterval = 500 * time.Millisecond

// startDevWatcher begins polling the on-disk copy of server/web for changes.
// Any time the directory fingerprint flips we notify all reload subscribers via
// the provided notifier. The returned cancel function stops the watcher.
func startDevWatcher(root string, notifier *reloadNotifier) context.CancelFunc {
	ctx, cancel := context.WithCancel(context.Background())

	go func() {
		// Ensure there is a final notification when the watcher stops so any open
		// SSE connections can exit rather than hanging indefinitely.
		defer notifier.Notify()

		lastFingerprint, err := directoryFingerprint(root)
		if err != nil {
			slog.Error("dev reload watcher failed to read directory", slog.String("root", root), slog.Any("err", err))
		}

		ticker := time.NewTicker(devWatcherInterval)
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				fp, err := directoryFingerprint(root)
				if err != nil {
					slog.Error("dev reload watcher failed to scan directory", slog.String("root", root), slog.Any("err", err))
					continue
				}

				if fp != lastFingerprint {
					lastFingerprint = fp
					// Directory changed; broadcast to all listeners.
					notifier.Notify()
				}
			}
		}
	}()

	return cancel
}

// directoryFingerprint produces a deterministic hash for the current state of
// the directory that includes relative path, file size, and modification time.
// It lets us cheaply detect meaningful changes without reading entire files.
func directoryFingerprint(root string) (string, error) {
	hasher := sha1.New()

	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		if d.IsDir() {
			return nil
		}

		info, err := d.Info()
		if err != nil {
			return err
		}

		relative, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}

		if _, err = fmt.Fprintf(hasher, "%s:%d:%d;", relative, info.ModTime().UnixNano(), info.Size()); err != nil {
			return err
		}

		return nil
	})
	if err != nil {
		return "", err
	}

	return hex.EncodeToString(hasher.Sum(nil)), nil
}
