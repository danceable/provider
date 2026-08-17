package provider

import (
	"context"
	"sync"
)

// Scope is a live scoped instance: a child container seeded with the WithValue
// values and with the manager's scoped providers already registered and booted.
// Call Terminate to tear the scoped providers down in reverse order (unless
// WithAutoTermination was set, which does this for you on context cancellation).
//
// A scope is the manager's lifecycle in miniature: its own copy of the providers,
// run through the same three phases, against a child container.
type Scope struct {
	// name is the scope name; empty for an ephemeral scope.
	name string

	// container is the child container backing this scope, as handed to callers
	// and to the scoped providers.
	container Container

	// base is the same child container without the deferred-loading decoration.
	// A provider loading later is given its own decoration of it.
	base Container

	// providers is the scope's own copy of the manager's scoped providers, with
	// its own state: what this scope has booted, and which of its deferred
	// providers it has loaded.
	providers *registry

	// deferred is the view of the providers this scope has yet to load.
	deferred *loading

	// termOnce guards termination so it runs exactly once.
	termOnce sync.Once

	// termErr is the result of the single termination run.
	termErr error

	// done is closed when termination has run, signalling the auto-termination
	// watcher to stop.
	done chan struct{}
}

// newScope creates a scope over the given child container and its own copy of
// the scoped providers. The container is decorated only when the scope has
// something to defer, keeping the plain path free of the decoration.
func newScope(name string, container Container, providers *registry, ctx context.Context) *Scope {
	scope := &Scope{
		name:      name,
		container: container,
		base:      container,
		providers: providers,
		done:      make(chan struct{}),
	}
	scope.deferred = newLoading(providers, ctx, scope.load)

	if scope.deferred.any() {
		scope.container = newDeferring(container, scope.deferred)
	}

	return scope
}

// Name returns the scope name, or an empty string for an ephemeral scope.
func (s *Scope) Name() string { return s.name }

// Container returns the child container backing the scope.
func (s *Scope) Container() Container { return s.container }

// Terminate terminates the scope's providers in reverse order, mirroring the
// manager's global termination semantics. It is idempotent: only the first call
// runs the providers' Terminate, and every call returns that run's error.
func (s *Scope) Terminate(ctx context.Context) error {
	s.termOnce.Do(func() {
		close(s.done)
		s.termErr = terminate(ctx, s.providers)
	})

	return s.termErr
}

// open registers and boots the scope's providers, leaving the deferred ones to
// the first request that needs them.
func (s *Scope) open(ctx context.Context) error {
	if err := register(ctx, s.providers, s.container); err != nil {
		return err
	}

	return boot(ctx, s.providers, s.container)
}

// load runs a deferred provider through its two phases against the scope's
// container. The provider works through the nested view, so requesting one of
// its own types while it registers or boots does not make it wait for itself.
func (s *Scope) load(ctx context.Context, entry *registration, nested *loading) error {
	return load(ctx, s.providers, entry, newDeferring(s.base, nested))
}

// watch terminates the scope when ctx is cancelled, or stops once the scope has
// already been terminated. The teardown context drops ctx's cancellation (it is
// already done) while preserving its values for cleanup.
func (s *Scope) watch(ctx context.Context) {
	select {
	case <-ctx.Done():
		_ = s.Terminate(context.WithoutCancel(ctx))
	case <-s.done:
	}
}
