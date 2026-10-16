package mobile

import "sync"

type commandKind uint8

const (
	configure commandKind = iota
	pause
	back
	cancelInput
	importCompleted
	exportCompleted
)

type command struct {
	kind      commandKind
	directory string
	completed chan struct{}
	message   string
}

// commandQueue retains UI/lifecycle requests while the render surface is stopped.
// A short mutex protects only transfer; game work runs after taking a batch.
type commandQueue struct {
	mu      sync.Mutex
	pending []command
}

func (q *commandQueue) enqueue(request command) {
	q.mu.Lock()
	q.pending = append(q.pending, request)
	q.mu.Unlock()
}

func (q *commandQueue) drain() []command {
	q.mu.Lock()
	requests := q.pending
	q.pending = nil
	q.mu.Unlock()
	return requests
}
