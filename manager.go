package provider

import (
	"context"
	"sync/atomic"
)

// Manager manages the lifecycle of service providers, including their registration, booting, and termination.
type Manager struct {
	providers *registry

	// scopedProviders is the template every scope copies.
	scopedProviders *registry

	container Container
	run       runContext
	options   *options
}

// New creates a new instance of the service provider manager with the given container.
func New(container Container) *Manager {
	return &Manager{
		providers:       newRegistry(),
		scopedProviders: newRegistry(),
		container:       container,
		options:         DefaultOptions(),
	}
}

// Register registers a service provider with the service provider manager.
// Providers that implement HasScope and return true are stored as scoped
// providers (run per-scope); all others run at global boot. One that also
// implements Deferrable keeps its place but runs on first use instead of at boot.
func (m *Manager) Register(provider Provider) {
	if scoped(provider) {
		m.scopedProviders.add(provider, provides(provider))

		return
	}

	m.providers.add(provider, provides(provider))
}

// Run registers and boots the providers, waits for ctx to be cancelled, then
// terminates them.
func (m *Manager) Run(ctx context.Context, opts ...Option) error {
	for _, opt := range opts {
		opt(m.options)
	}

	m.run.start(ctx)

	container := m.containerFor(ctx)

	if err := register(ctx, m.providers, container); err != nil {
		return err
	}

	if err := boot(ctx, m.providers, container); err != nil {
		return err
	}

	if m.options.Callback != nil {
		go m.options.Callback(ctx, container)
	}

	<-ctx.Done()

	return shutdown(m.providers, m.options)
}

// Scope opens a scoped instance of the container and runs the manager's scoped
// providers against it. By default the scope is anonymous and ephemeral
// (container.Derive); WithPersistent makes it a named, persistent child instead.
// The caller owns the returned Scope and must Terminate it, unless
// WithAutoTermination ties teardown to ctx.
//
// On any error the scope is not returned; matching Run, already-booted providers
// are not terminated here.
func (m *Manager) Scope(ctx context.Context, opts ...ScopeOption) (*Scope, error) {
	config := newScopeConfig(opts)

	container, err := m.child(ctx, config)
	if err != nil {
		return nil, err
	}

	scope := newScope(config.name, container, m.scopedProviders.clone(), ctx)

	if err := scope.open(ctx); err != nil {
		return nil, err
	}

	if config.autoTerminate {
		go scope.watch(ctx)
	}

	return scope, nil
}

// child creates the container backing a scope, seeded with the configured
// values. It comes from the manager's own container, so a globally deferred type
// requested inside the scope still loads its provider.
func (m *Manager) child(ctx context.Context, config *scopeConfig) (Container, error) {
	parent := m.containerFor(ctx)

	var container Container
	if config.persistent {
		container = parent.Scope(config.name)
	} else {
		container = parent.Derive()
	}

	for _, v := range config.values {
		if err := bindValue(container, v.name, v.value); err != nil {
			return nil, err
		}
	}

	return container, nil
}

func (m *Manager) containerFor(ctx context.Context) Container {
	return newLoading(m.providers, m.container, m.run.contextOr(ctx)).container()
}

// runContext holds the context Run was started with. Deferred providers are
// loaded with it, so their lifetime follows the application's rather than that
// of whoever triggered the load.
type runContext struct {
	ctx atomic.Pointer[context.Context]
}

func (r *runContext) start(ctx context.Context) {
	r.ctx.Store(&ctx)
}

func (r *runContext) contextOr(fallback context.Context) context.Context {
	if ctx := r.ctx.Load(); ctx != nil {
		return *ctx
	}

	return fallback
}
