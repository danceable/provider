package provider

import (
	"context"
	"maps"
	"slices"
	"sync"

	"github.com/danceable/container/bind"
	"github.com/danceable/container/resolve"
)

// Container defines the interface for a dependency injection container.
type Container interface {
	// Reset calls the same method of the default concrete.
	Reset()

	// Bind calls the same method of the default concrete.
	Bind(receiver any, opts ...bind.BindOption) error

	// Call calls the same method of the default concrete.
	Call(receiver any, opts ...resolve.ResolveOption) error

	// Resolve calls the same method of the default concrete.
	Resolve(abstraction any, opts ...resolve.ResolveOption) error

	// Fill calls the same method of the default concrete.
	Fill(receiver any, opts ...resolve.ResolveOption) error
}

// Provider defines the interface for a service provider.
type Provider interface {
	// Register registers the provider's services into the container.
	// This method is called during the application's initialization phase.
	Register(ctx context.Context, container Container) error

	// Boot boots the provider, which is called after all providers have been registered.
	// This method is used to perform any initialization tasks that require access to other providers.
	Boot(ctx context.Context, container Container) error

	// Terminate terminates the provider, which is called before the application exits.
	// This method is used to release resources or perform cleanup tasks.
	Terminate() error
}

// HasOrder is an optional interface that providers can implement to specify their execution order.
type HasOrder interface {

	// Order determines the execution order of the provider.
	// Providers with lower order values are registered and booted before those with higher values.
	// 1- first register from lower to higher.
	// 2- then boot from lower to higher.
	// 3- finally terminate from higher to lower. (reverse order for termination)
	Order() int
}

// Manager manages the lifecycle of service providers, including their registration, booting, and termination.
type Manager struct {
	// providers holds the registered service providers.
	providers map[int][]Provider

	// container is the dependency injection container used to manage service instances.
	container Container

	// sortedProvidersCache is a cache of sorted provider keys which specifies the order of execution.
	sortedProvidersCache []int

	// mu is a mutex to protect the state of the manager.
	mu sync.RWMutex
}

// New creates a new instance of the service provider manager with the given container.
func New(container Container) *Manager {
	return &Manager{
		providers: make(map[int][]Provider),
		container: container,
	}
}

// Register registers a service provider with the service provider manager.
func (m *Manager) Register(provider Provider) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if hasOrder, ok := provider.(HasOrder); ok {
		order := hasOrder.Order()
		m.providers[order] = append(m.providers[order], provider)

		return
	}

	m.providers[len(m.providers)] = append(m.providers[len(m.providers)], provider)
}

// Run executes the service provider manager, which involves booting all registered providers and handling their termination.
func (m *Manager) Run(ctx context.Context) error {
	m.refreshSortedProvidersCache()

	if err := m.register(ctx); err != nil {
		return err
	}

	if err := m.boot(ctx); err != nil {
		return err
	}

	// wait for a signal to terminate the providers.
	<-ctx.Done()

	if err := m.terminate(); err != nil {
		return err
	}

	return nil
}

func (m *Manager) register(ctx context.Context) error {
	for _, order := range m.sortedProvidersCache {
		providers := m.providers[order]
		for _, provider := range providers {
			if err := provider.Register(ctx, m.container); err != nil {
				return err
			}
		}
	}

	return nil
}

func (m *Manager) boot(ctx context.Context) error {
	for _, order := range m.sortedProvidersCache {
		providers := m.providers[order]
		for _, provider := range providers {
			if err := provider.Boot(ctx, m.container); err != nil {
				return err
			}
		}
	}

	return nil
}

func (m *Manager) terminate() error {
	m.mu.RLock()
	defer m.mu.RUnlock()

	// Terminate providers in reverse order
	for i := range slices.Backward(m.sortedProvidersCache) {
		order := m.sortedProvidersCache[i]
		providers := m.providers[order]
		for j := range slices.Backward(providers) {
			provider := providers[j]
			if err := provider.Terminate(); err != nil {
				return err
			}
		}
	}

	return nil
}

func (m *Manager) refreshSortedProvidersCache() {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.sortedProvidersCache = slices.Sorted(maps.Keys(m.providers))
}
