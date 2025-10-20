package server

import (
	"sync"
)

// reloadNotifier fan-outs change notifications to any number of subscribers.
// Each subscriber gets a buffered channel that receives a single empty struct
// whenever a change occurs.
type reloadNotifier struct {
	mu      sync.Mutex
	closed  bool
	nextID  int
	clients map[int]chan struct{}
}

func newReloadNotifier() *reloadNotifier {
	return &reloadNotifier{
		clients: make(map[int]chan struct{}),
	}
}

// Subscribe registers a new listener and returns a cancellation function along
// with the channel that delivers reload signals. Callers must invoke the
// returned function once they are done listening so the notifier can reclaim
// resources. If the notifier has already been closed we return a nil channel.
func (n *reloadNotifier) Subscribe() (func(), <-chan struct{}) {
	n.mu.Lock()
	defer n.mu.Unlock()

	if n.closed {
		return func() {}, nil
	}

	id := n.nextID
	n.nextID++

	ch := make(chan struct{}, 1)
	n.clients[id] = ch

	var once sync.Once

	cancel := func() {
		once.Do(func() {
			n.mu.Lock()
			defer n.mu.Unlock()

			if ch, ok := n.clients[id]; ok {
				close(ch)
				delete(n.clients, id)
			}
		})
	}

	return cancel, ch
}

// Notify broadcasts a reload signal to every active listener without blocking
// on slow readers. If a listener already has a pending notification we leave it
// untouched so it still reloads on its next poll.
func (n *reloadNotifier) Notify() {
	n.mu.Lock()
	defer n.mu.Unlock()

	if n.closed {
		return
	}

	for _, ch := range n.clients {
		select {
		case ch <- struct{}{}:
		default:
			// channel already has pending notification; skip
		}
	}
}

// Close tears down the notifier and closes every subscriber channel, signalling
// to callers that no further reload events will arrive.
func (n *reloadNotifier) Close() {
	n.mu.Lock()
	defer n.mu.Unlock()

	if n.closed {
		return
	}

	n.closed = true

	for id, ch := range n.clients {
		close(ch)
		delete(n.clients, id)
	}
}
