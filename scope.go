package provider

import (
	"context"
	"errors"
	"maps"
	"reflect"
	"slices"
	"sync"

	"github.com/danceable/container/bind"
)

// ErrNilScopeValue is returned when a nil value is passed to WithValue. The
// container binds values by their reflected type, which cannot be determined
// from an untyped nil.
var ErrNilScopeValue = errors.New("provider: scope value must not be nil")

// scopeConfig collects the per-scope configuration produced by ScopeOptions.
type scopeConfig struct {
	// values are bound into the scoped container before its providers run.
	values []scopedValue

	// name is the scope name; only used when persistent is true.
	name string

	// persistent makes the scope a named, persistent child instead of an
	// anonymous, ephemeral one.
	persistent bool

	// autoTerminate ties the scope's teardown to the context: the scope is
	// terminated automatically once the context passed to Scope is cancelled.
	autoTerminate bool
}

// scopedValue is a single named value seeded into a scoped container.
type scopedValue struct {
	name  string
	value any
}

// ScopeOption configures a scoped instance of the container.
type ScopeOption func(*scopeConfig)

// WithValue seeds the scoped container with value, resolvable by name. The
// value is bound as a named singleton, so scoped providers (and anything else
// resolving from the scope) can retrieve it via resolve.WithName(name).
func WithValue(name string, value any) ScopeOption {
	return func(c *scopeConfig) {
		c.values = append(c.values, scopedValue{name: name, value: value})
	}
}

// WithPersistent makes the scope a named, persistent child of the manager's
// container (container.Scope) rather than the default anonymous, ephemeral one
// (container.Derive). The named child is cached on its parent and reused by
// later calls with the same name.
func WithPersistent(name string) ScopeOption {
	return func(c *scopeConfig) {
		c.persistent = true
		c.name = name
	}
}

// WithAutoTermination makes the scope terminate itself once the context passed
// to Scope is cancelled, releasing the caller from calling Terminate. Teardown
// runs exactly once, whether triggered by the context or by an explicit
// Terminate, so combining the two is safe.
func WithAutoTermination() ScopeOption {
	return func(c *scopeConfig) {
		c.autoTerminate = true
	}
}

// Scope is a live scoped instance: a child container seeded with the WithValue
// values and with the manager's scoped providers already registered and booted.
// Call Terminate to tear the scoped providers down in reverse order (unless
// WithAutoTermination was set, which does this for you on context cancellation).
type Scope struct {
	// name is the scope name; empty for an ephemeral scope.
	name string

	// container is the child container backing this scope.
	container Container

	// sorted holds the scoped provider orders, lowest first.
	sorted []int

	// providers is the snapshot of scoped providers taken when the scope was created.
	providers map[int][]Provider

	// termOnce guards termination so it runs exactly once.
	termOnce sync.Once

	// termErr is the result of the single termination run.
	termErr error

	// done is closed when termination has run, signalling the auto-termination
	// watcher to stop.
	done chan struct{}
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
		s.termErr = s.terminate(ctx)
	})

	return s.termErr
}

// terminate runs the scoped providers' Terminate in reverse order.
func (s *Scope) terminate(ctx context.Context) error {
	for i := range slices.Backward(s.sorted) {
		providers := s.providers[s.sorted[i]]
		for j := range slices.Backward(providers) {
			if err := providers[j].Terminate(ctx); err != nil {
				return err
			}
		}
	}

	return nil
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

// Scope opens a scoped instance of the container and runs the manager's scoped
// providers against it. By default the scope is anonymous and ephemeral
// (container.Derive), becoming eligible for garbage collection once the caller
// drops the returned Scope; WithPersistent makes it a named, persistent child
// instead. Any WithValue options seed the child before the scoped providers'
// Register then Boot. The caller owns the returned Scope and must Terminate it
// when the scope ends, unless WithAutoTermination ties teardown to ctx.
//
// On any error the scope is not returned; matching the manager's global Run,
// already-booted providers are not terminated here.
func (m *Manager) Scope(ctx context.Context, opts ...ScopeOption) (*Scope, error) {
	config := &scopeConfig{}
	for _, opt := range opts {
		opt(config)
	}

	var (
		name      string
		container Container
	)
	if config.persistent {
		name = config.name
		container = m.container.Scope(name)
	} else {
		container = m.container.Derive()
	}

	for _, v := range config.values {
		if err := bindValue(container, v.name, v.value); err != nil {
			return nil, err
		}
	}

	sorted, providers := m.snapshotScopedProviders()
	scope := &Scope{
		name:      name,
		container: container,
		sorted:    sorted,
		providers: providers,
		done:      make(chan struct{}),
	}

	for _, order := range scope.sorted {
		for _, provider := range scope.providers[order] {
			if err := provider.Register(ctx, container); err != nil {
				return nil, err
			}
		}
	}

	for _, order := range scope.sorted {
		for _, provider := range scope.providers[order] {
			if err := provider.Boot(ctx, container); err != nil {
				return nil, err
			}
		}
	}

	if config.autoTerminate {
		go scope.watch(ctx)
	}

	return scope, nil
}

// snapshotScopedProviders returns, under a single lock, the scoped provider
// orders (lowest first) and a copy of the scoped providers map. The copy keeps
// a live scope unaffected by concurrent Register calls.
func (m *Manager) snapshotScopedProviders() ([]int, map[int][]Provider) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	providers := make(map[int][]Provider, len(m.scopedProviders))
	for order, ps := range m.scopedProviders {
		providers[order] = slices.Clone(ps)
	}

	return slices.Sorted(maps.Keys(m.scopedProviders)), providers
}

// bindValue binds value into the container as a named singleton. The container
// only accepts function resolvers, so the value is wrapped in a generated
// func() T returning it.
func bindValue(c Container, name string, value any) error {
	v := reflect.ValueOf(value)
	if !v.IsValid() {
		return ErrNilScopeValue
	}

	resolver := reflect.MakeFunc(
		reflect.FuncOf(nil, []reflect.Type{v.Type()}, false),
		func([]reflect.Value) []reflect.Value { return []reflect.Value{v} },
	)

	return c.Bind(resolver.Interface(), bind.WithName(name), bind.Singleton())
}
