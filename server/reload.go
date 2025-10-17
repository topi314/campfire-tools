package server

import (
	"sync"
)

// reloadNotifier fan-outs change notifications to any number of subscribers.
// Each subscriber gets a buffered channel that receives a single empty struct
// whenever a change occurs.
type reloadNotifier struct {
	mutex   sync.Mutex
	closed  bool
	nextID  int
	clients map[int]chan struct{}
}

func newReloadNotifier() *reloadNotifier {
	return &reloadNotifier{
		clients: make(map[int]chan struct{}),
	}
}

// Subscribe registers a new listener and returns both its ID and a channel that
// delivers reload signals. If the notifier has already been closed we return a
// closed channel so callers can fail fast.
func (notifier *reloadNotifier) Subscribe() (int, <-chan struct{}) {
	notifier.mutex.Lock()
	defer notifier.mutex.Unlock()

	if notifier.closed {
		ch := make(chan struct{})
		close(ch)
		return -1, ch
	}

	id := notifier.nextID
	notifier.nextID++

	ch := make(chan struct{}, 1)
	notifier.clients[id] = ch

	return id, ch
}

// Unsubscribe removes an existing listener and closes its channel so the caller
// can tear down any goroutines blocked on it.
func (notifier *reloadNotifier) Unsubscribe(id int) {
	notifier.mutex.Lock()
	defer notifier.mutex.Unlock()

	if ch, ok := notifier.clients[id]; ok {
		close(ch)
		delete(notifier.clients, id)
	}
}

// Notify broadcasts a reload signal to every active listener without blocking
// on slow readers. If a listener already has a pending notification we leave it
// untouched so it still reloads on its next poll.
func (notifier *reloadNotifier) Notify() {
	notifier.mutex.Lock()
	defer notifier.mutex.Unlock()

	if notifier.closed {
		return
	}

	for _, ch := range notifier.clients {
		select {
		case ch <- struct{}{}:
		default:
			// channel already has pending notification; skip
		}
	}
}

// Close tears down the notifier and closes every subscriber channel, signalling
// to callers that no further reload events will arrive.
func (notifier *reloadNotifier) Close() {
	notifier.mutex.Lock()
	defer notifier.mutex.Unlock()

	if notifier.closed {
		return
	}

	notifier.closed = true

	for id, ch := range notifier.clients {
		close(ch)
		delete(notifier.clients, id)
	}
}
