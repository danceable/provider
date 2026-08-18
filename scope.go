package provider

import (
	"context"
	"sync"
)

// Scope is a live scoped instance: a child container seeded with the WithValue
// values and with the manager's scoped providers already registered and booted.
// Call Terminate to tear them down in reverse order, unless WithAutoTermination
// was set.
type Scope struct {
	name      string
	container Container
	providers *registry

	termOnce sync.Once
	termErr  error

	// done is closed once termination has run, stopping the watcher.
	done chan struct{}
}

func newScope(name string, container Container, providers *registry, ctx context.Context) *Scope {
	deferred := newLoading(providers, container, ctx)

	return &Scope{
		name:      name,
		container: deferred.container(),
		providers: providers,
		done:      make(chan struct{}),
	}
}

// Name returns the scope name, or an empty string for an ephemeral scope.
func (s *Scope) Name() string { return s.name }

// Container returns the child container backing the scope.
func (s *Scope) Container() Container { return s.container }

// Terminate terminates the scope's providers in reverse order. It is idempotent:
// only the first call runs them, and every call returns that run's error.
func (s *Scope) Terminate(ctx context.Context) error {
	s.termOnce.Do(func() {
		close(s.done)
		s.termErr = terminate(ctx, s.providers)
	})

	return s.termErr
}

func (s *Scope) open(ctx context.Context) error {
	if err := register(ctx, s.providers, s.container); err != nil {
		return err
	}

	return boot(ctx, s.providers, s.container)
}

// watch terminates the scope once ctx is cancelled. The teardown context drops
// that cancellation while keeping its values.
func (s *Scope) watch(ctx context.Context) {
	select {
	case <-ctx.Done():
		_ = s.Terminate(context.WithoutCancel(ctx))
	case <-s.done:
	}
}
