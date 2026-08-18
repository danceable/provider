package provider

import "sync"

// gate runs a function once and makes the callers that arrive while it runs wait
// for that single run. Every caller receives the error of the one run; whoever
// must not wait — the run itself asking again — simply never calls do.
type gate struct {
	started bool
	done    chan struct{}
	err     error

	mu sync.Mutex
}

func newGate() *gate {
	return &gate{done: make(chan struct{})}
}

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
