package watcher

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/fsnotify/fsnotify"
)

const defaultCapacity = 256

type Watcher struct {
	root     string
	display  string
	started  time.Time
	capacity int

	backend *fsnotify.Watcher
	done    chan struct{}
	wg      sync.WaitGroup
	once    sync.Once

	mu     sync.RWMutex
	nextID int64
	events []EventRecord
}

type EventRecord struct {
	ID         int64
	ObservedAt time.Time
	Op         string
	Path       string
	FullPath   string
	Directory  bool
	Message    string
}

type Snapshot struct {
	Root           string
	StartedAt      time.Time
	EventCount     int64
	BufferCapacity int
}

// New starts a recursive directory watcher.
func New(root string) (*Watcher, error) {
	abs, err := filepath.Abs(root)
	if err != nil {
		return nil, err
	}

	info, err := os.Stat(abs)
	if err != nil {
		return nil, fmt.Errorf("stat watch path: %w", err)
	}
	if !info.IsDir() {
		return nil, fmt.Errorf("watch path is not a directory: %s", abs)
	}

	backend, err := fsnotify.NewWatcher()
	if err != nil {
		return nil, err
	}

	w := &Watcher{
		root:     abs,
		display:  filepath.Base(abs),
		started:  time.Now(),
		capacity: defaultCapacity,
		backend:  backend,
		done:     make(chan struct{}),
	}

	if err := w.addExistingDirectories(); err != nil {
		_ = backend.Close()
		return nil, err
	}

	w.wg.Add(1)
	go w.run()
	return w, nil
}

func (w *Watcher) Root() string {
	return w.root
}

func (w *Watcher) DisplayRoot() string {
	if w.display == "" || w.display == "." || w.display == string(filepath.Separator) {
		return w.root
	}
	return w.display
}

func (w *Watcher) Snapshot() Snapshot {
	w.mu.RLock()
	defer w.mu.RUnlock()

	return Snapshot{
		Root:           w.root,
		StartedAt:      w.started,
		EventCount:     w.nextID,
		BufferCapacity: w.capacity,
	}
}

func (w *Watcher) Events() []EventRecord {
	w.mu.RLock()
	defer w.mu.RUnlock()

	events := make([]EventRecord, len(w.events))
	copy(events, w.events)
	return events
}

func (w *Watcher) Close() error {
	var err error
	w.once.Do(func() {
		close(w.done)
		err = w.backend.Close()
		w.wg.Wait()
	})
	return err
}

func (w *Watcher) addExistingDirectories() error {
	return filepath.WalkDir(w.root, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !entry.IsDir() {
			return nil
		}
		if err := w.backend.Add(path); err != nil {
			return fmt.Errorf("watch directory %s: %w", path, err)
		}
		return nil
	})
}

func (w *Watcher) run() {
	defer w.wg.Done()

	for {
		select {
		case event, ok := <-w.backend.Events:
			if !ok {
				return
			}
			w.handleEvent(event)
		case err, ok := <-w.backend.Errors:
			if !ok {
				return
			}
			w.appendEvent(EventRecord{
				ObservedAt: time.Now(),
				Op:         "error",
				Message:    err.Error(),
			})
		case <-w.done:
			return
		}
	}
}

func (w *Watcher) handleEvent(event fsnotify.Event) {
	if event.Name == "" {
		return
	}

	if event.Has(fsnotify.Create) {
		if info, err := os.Stat(event.Name); err == nil && info.IsDir() {
			if err := w.addDirectoryTree(event.Name); err != nil {
				w.appendEvent(EventRecord{
					ObservedAt: time.Now(),
					Op:         "error",
					Path:       w.relativePath(event.Name),
					FullPath:   event.Name,
					Message:    err.Error(),
				})
			}
		}
	}

	info, statErr := os.Stat(event.Name)
	w.appendEvent(EventRecord{
		ObservedAt: time.Now(),
		Op:         describeOp(event.Op),
		Path:       w.relativePath(event.Name),
		FullPath:   event.Name,
		Directory:  statErr == nil && info.IsDir(),
	})
}

func (w *Watcher) addDirectoryTree(root string) error {
	return filepath.WalkDir(root, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !entry.IsDir() {
			return nil
		}
		return w.backend.Add(path)
	})
}

func (w *Watcher) appendEvent(event EventRecord) {
	w.mu.Lock()
	defer w.mu.Unlock()

	w.nextID++
	event.ID = w.nextID
	if event.ObservedAt.IsZero() {
		event.ObservedAt = time.Now()
	}

	w.events = append(w.events, event)
	if len(w.events) > w.capacity {
		copy(w.events, w.events[1:])
		w.events = w.events[:w.capacity]
	}
}

func (w *Watcher) relativePath(name string) string {
	rel, err := filepath.Rel(w.root, name)
	if err != nil || rel == "." {
		return filepath.Base(name)
	}
	return rel
}

func describeOp(op fsnotify.Op) string {
	value := strings.ToLower(op.String())
	if value == "" {
		return "unknown"
	}
	return value
}
