package provider

import (
	"context"
	"reflect"
)

// Provider defines the interface for a service provider.
type Provider interface {
	// Register registers the provider's services into the container.
	Register(ctx context.Context, container Container) error

	// Boot runs after all providers have registered, for initialization that
	// depends on their bindings.
	Boot(ctx context.Context, container Container) error

	// Terminate releases what the provider holds. It is called for every provider
	// whose Register ran, including one whose Boot failed, so it must tolerate a
	// partially initialized provider.
	Terminate(ctx context.Context) error
}

// The interfaces below are optional: a provider implements one to change where,
// when or in what order it runs. The helpers next to them are the only place
// that knows a provider may implement more than Provider.

// HasOrder is an optional interface for providers that specify their execution
// order. Lower orders register and boot first, and terminate last.
type HasOrder interface {
	Order() int
}

func order(provider Provider) (int, bool) {
	hasOrder, ok := provider.(HasOrder)
	if !ok {
		return 0, false
	}

	return hasOrder.Order(), true
}

// HasScope is an optional interface for providers that run per-scope instead of
// at global boot.
type HasScope interface {
	Scoped() bool
}

func scoped(provider Provider) bool {
	hasScope, ok := provider.(HasScope)

	return ok && hasScope.Scoped()
}

// Deferrable is an optional interface for providers that defer their
// registration until one of the types they provide is actually needed.
//
// The phase that would run such a provider walks past it; it is registered and
// booted the first time one of its types is requested from the container,
// through Resolve, Call or Fill, and never at all if nothing asks. Loading is
// depth-first: the provider is completely up, including anything it requests in
// turn, before the request that triggered it carries on. An empty result opts
// out of deferral.
type Deferrable interface {
	Provides() []reflect.Type
}

func provides(provider Provider) []reflect.Type {
	deferrable, ok := provider.(Deferrable)
	if !ok {
		return nil
	}

	return deferrable.Provides()
}
