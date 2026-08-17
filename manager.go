package provider

import (
	"context"
	"sync"
)

// Manager manages the lifecycle of service providers, including their registration, booting, and termination.
//
// It owns two sets of providers: the ones it runs itself, and the scoped ones
// every scope copies. Both are ordinary registries and both go through the same
// three phases; the manager decides which providers go where and with what
// container, and leaves the phases to run them.
type Manager struct {
	// providers holds the registered service providers.
	providers *registry

	// scopedProviders holds the providers that run per-scope instead of at
	// global boot. It is the template every scope copies.
	scopedProviders *registry

	// container is the dependency injection container used to manage service instances.
	container Container

	// runCtx is the context Run was started with, once it is running.
	runCtx context.Context

	// options holds the configuration
	options *options

	// mu is a mutex to protect the state of the manager.
	mu sync.RWMutex
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
// providers (run per-scope); all others run at global boot.
//
// A provider that also implements Deferrable keeps its place among the others
// but waits there: instead of running at boot, it is registered and booted the
// first time one of the types it provides is requested from the container.
func (m *Manager) Register(provider Provider) {
	if scoped(provider) {
		m.scopedProviders.add(provider, provides(provider))

		return
	}

	m.providers.add(provider, provides(provider))
}

// Run executes the service provider manager, which involves booting all registered providers and handling their termination.
func (m *Manager) Run(ctx context.Context, opts ...Option) error {
	for _, opt := range opts {
		opt(m.options)
	}

	m.mu.Lock()
	m.runCtx = ctx
	m.mu.Unlock()

	container := m.deferringContainer(ctx)

	if err := register(ctx, m.providers, container); err != nil {
		return err
	}

	if err := boot(ctx, m.providers, container); err != nil {
		return err
	}

	if m.options.Callback != nil {
		go m.options.Callback(ctx, container)
	}

	// wait for a signal to terminate the providers.
	<-ctx.Done()

	return shutdown(m.providers, m.options)
}

// Scope opens a scoped instance of the container and runs the manager's scoped
// providers against it. By default the scope is anonymous and ephemeral
// (container.Derive), becoming eligible for garbage collection once the caller
// drops the returned Scope; WithPersistent makes it a named, persistent child
// instead. Any WithValue options seed the child before the scoped providers'
// Register then Boot. The caller owns the returned Scope and must Terminate it
// when the scope ends, unless WithAutoTermination ties teardown to ctx.
//
// Scoped providers that are also Deferrable are not run here: each scope gets
// its own copy of them and loads it the first time the scope's container is
// asked for one of the types it provides.
//
// On any error the scope is not returned; matching the manager's global Run,
// already-booted providers are not terminated here.
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

// child creates the container backing a scope: a named, persistent child or an
// anonymous, ephemeral one, seeded with the configured values. It is taken from
// the deferring container, so requesting a globally deferred type from inside
// the scope still loads its provider.
func (m *Manager) child(ctx context.Context, config *scopeConfig) (Container, error) {
	parent := m.deferringContainer(ctx)

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

// load runs a deferred provider through its two phases against the manager's
// container. The provider works through the nested view, so requesting another
// deferred type while it loads triggers that provider in turn.
func (m *Manager) load(ctx context.Context, entry *registration, nested *loading) error {
	return load(ctx, m.providers, entry, newDeferring(m.container, nested))
}

// deferringContainer wraps the manager's container so that requesting a deferred
// type loads its provider first. The container is returned untouched when
// nothing is deferred, keeping the plain path free of the decoration.
func (m *Manager) deferringContainer(ctx context.Context) Container {
	deferred := newLoading(m.providers, m.baseContext(ctx), m.load)
	if !deferred.any() {
		return m.container
	}

	return newDeferring(m.container, deferred)
}

// baseContext returns the context deferred providers are loaded with: the one
// Run was started with, when the manager is running. A deferred provider lives
// as long as the application does, so it must not inherit the context of the
// request that happened to trigger it.
func (m *Manager) baseContext(ctx context.Context) context.Context {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if m.runCtx != nil {
		return m.runCtx
	}

	return ctx
}
