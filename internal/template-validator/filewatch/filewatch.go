package filewatch

import (
	"fmt"
	"strings"
	"sync"
	"sync/atomic"

	"github.com/fsnotify/fsnotify"
)

type Watch interface {
	Add(path string, callback func()) error
	Run(done <-chan struct{}) error
	IsRunning() bool
}

func New() Watch {
	return &watch{
		callbacks: make(map[string]func()),
	}
}

type watch struct {
	lock      sync.Mutex
	callbacks map[string]func()
	running   atomic.Bool
}

var _ Watch = &watch{}

func (w *watch) Add(path string, callback func()) error {
	w.lock.Lock()
	defer w.lock.Unlock()

	if w.running.Load() {
		return fmt.Errorf("cannot add to a running watch")
	}

	w.callbacks[path] = callback
	return nil
}

func (w *watch) Run(done <-chan struct{}) error {
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return fmt.Errorf("could not create fsnotify.Watcher: %w", err)
	}
	// watcher.Close() never returns an error
	defer func() { _ = watcher.Close() }()

	func() {
		// Before setting running to true, we need to acquire the lock,
		// because Add() method may be running concurrently.
		w.lock.Lock()
		defer w.lock.Unlock()
		w.running.Store(true)
	}()
	// Setting running to false is ok without a lock.
	defer w.running.Store(false)

	err = w.addCallbacks(watcher)
	if err != nil {
		return fmt.Errorf("could not add callbacks: %w", err)
	}
	// Running all callbacks before processing watch events.
	// So callbacks will notice the state of the files after
	// watch starts, but before any events arrive.
	w.runCallbacks()

	return w.processEvents(watcher, done)
}

func (w *watch) IsRunning() bool {
	return w.running.Load()
}

func (w *watch) addCallbacks(watcher *fsnotify.Watcher) error {
	for path := range w.callbacks {
		err := watcher.Add(path)
		if err != nil {
			return fmt.Errorf("failed watch %s: %w", path, err)
		}
	}
	return nil
}

func (w *watch) runCallbacks() {
	for _, callback := range w.callbacks {
		callback()
	}
}

func (w *watch) processEvents(watcher *fsnotify.Watcher, done <-chan struct{}) error {
	for {
		select {
		case <-done:
			return nil

		case event, ok := <-watcher.Events:
			if !ok {
				return nil
			}
			w.handleEvent(event)

		case err, ok := <-watcher.Errors:
			if !ok {
				return nil
			}
			if err != nil {
				return err
			}
		}
	}
}

func (w *watch) handleEvent(event fsnotify.Event) {
	const modificationEvents = fsnotify.Create | fsnotify.Write | fsnotify.Remove
	if event.Op&modificationEvents == 0 {
		return
	}

	for path, callback := range w.callbacks {
		if strings.HasPrefix(event.Name, path) {
			callback()
		}
	}
}
