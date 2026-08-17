package provider

import "sync"

// gate runs a function once and makes the callers that arrive while it runs wait
// for that single run instead of racing past it. Every caller receives the error
// of the one run.
//
// It is sync.Once with the two properties this package needs: the result is
// reported to every caller, and waiting is the caller's choice — whoever must
// not wait, because it is the run itself asking again, simply never calls do.
type gate struct {
	// started reports whether a caller has taken the run; it is guarded by mu.
	started bool

	// done is closed once the run has finished, releasing the waiters.
	done chan struct{}

	// err is the result of the run; it is written before done is closed.
	err error

	// mu guards started.
	mu sync.Mutex
}

// newGate creates a gate that has not run yet.
func newGate() *gate {
	return &gate{done: make(chan struct{})}
}

// do runs f, or waits for the run already in progress, and returns its error.
// Only the first caller executes f.
func (g *gate) do(f func() error) error {
	g.mu.Lock()
	if g.started {
		g.mu.Unlock()
		<-g.done

		return g.err
	}
	g.started = true
	g.mu.Unlock()

	g.err = f()
	close(g.done)

	return g.err
}
