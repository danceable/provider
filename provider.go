package provider

import (
	"context"
	"reflect"
)

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
	//
	// It is called for every provider whose Register has run, including one whose
	// Boot failed or never happened, since Register alone may already have
	// acquired something. Terminate must therefore tolerate a partially
	// initialized provider and release only what it actually holds.
	Terminate(ctx context.Context) error
}

// The interfaces below are optional: a provider implements one to change a
// single aspect of how it is run — its position, where it runs, or when. They
// are queried through the small helpers next to them, which are the only place
// in the package that knows a provider may implement more than Provider.

// HasOrder is an optional interface that providers can implement to specify their execution order.
type HasOrder interface {

	// Order determines the execution order of the provider.
	// Providers with lower order values are registered and booted before those with higher values.
	// 1- first register from lower to higher.
	// 2- then boot from lower to higher.
	// 3- finally terminate from higher to lower. (reverse order for termination)
	Order() int
}

// order returns the execution order a provider declares, and whether it declares
// one at all.
func order(provider Provider) (int, bool) {
	hasOrder, ok := provider.(HasOrder)
	if !ok {
		return 0, false
	}

	return hasOrder.Order(), true
}

// HasScope is an optional interface that providers can implement to opt into
// scoped execution. When Register receives a provider whose Scoped method
// returns true, the provider is run per-scope (by Scope/Derive against a child
// container) instead of at global boot.
type HasScope interface {

	// Scoped reports whether the provider should be registered as a scoped provider.
	Scoped() bool
}

// scoped reports whether the provider opted into per-scope execution.
func scoped(provider Provider) bool {
	hasScope, ok := provider.(HasScope)

	return ok && hasScope.Scoped()
}

// Deferrable is an optional interface that providers can implement to defer
// their registration until one of the types they provide is actually needed.
//
// A deferred provider takes its usual place among the others — it keeps its
// order, and it is terminated with them — but the phase that would run it (Run,
// or Scope for a provider that is also scoped) walks past it. It is registered
// and booted the first time one of the types returned by Provides is requested
// from the container it belongs to, through Resolve, Call or Fill, and never at
// all if nothing asks for it. That keeps the boot path free of services the
// application may not use in a given run.
//
// Loading is depth-first: the request that triggers a provider blocks until it
// has both registered and booted, including any further deferred providers it
// requests along the way, so its dependencies are fully up before it binds
// anything.
//
// Returning an empty slice opts out of deferral: nothing could ever trigger the
// provider, so it runs at boot like any other.
type Deferrable interface {

	// Provides returns the types the provider binds into the container.
	// Requesting any of them loads the provider.
	Provides() []reflect.Type
}

// provides returns the types a provider defers on, and nothing for a provider
// that runs at boot.
func provides(provider Provider) []reflect.Type {
	deferrable, ok := provider.(Deferrable)
	if !ok {
		return nil
	}

	return deferrable.Provides()
}
