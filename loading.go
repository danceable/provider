package provider

import (
	"context"
	"reflect"
	"slices"
)

// loader registers and boots one deferred provider on behalf of the owner of a
// registry — the manager for its own providers, a scope for its copy of the
// scoped ones. It receives the nested view the provider must work through, so a
// provider that requests further deferred types while loading triggers them
// without waiting for itself.
type loader func(ctx context.Context, entry *registration, nested *loading) error

// loading is the deferred providers of a registry, as seen from one place in the
// container tree. It answers a single question — "does this request need a
// provider that has not run yet?" — and loads the ones that do.
//
// A view carries the context its owner was started with, because a container
// request has no context of its own, and the chain of providers already loading
// above it, so a provider is never made to wait for itself.
type loading struct {
	// providers is the set the view watches. Views derived from it share it.
	providers *registry

	// load registers and boots a single provider on behalf of the owner.
	load loader

	// ctx is the context deferred providers are loaded with: the one their owner
	// was started with, so a provider's lifetime follows its owner's rather than
	// that of whoever triggered the load.
	ctx context.Context

	// chain is the providers currently being loaded above this view.
	chain []*registration
}

var _ trigger = &loading{}

// newLoading creates the root view of a registry's deferred providers.
func newLoading(providers *registry, ctx context.Context, load loader) *loading {
	return &loading{providers: providers, load: load, ctx: ctx}
}

// any reports whether the view still has something to load. It is the cheap
// check that keeps an application without deferred providers on the plain path.
func (l *loading) any() bool {
	return l.providers.deferred()
}

// resolve loads every deferred provider triggered by the requested abstractions.
func (l *loading) resolve(abstractions ...reflect.Type) error {
	if !l.any() {
		return nil
	}

	for _, abstraction := range abstractions {
		if err := l.run(l.match(abstraction)); err != nil {
			return err
		}
	}

	return nil
}

// nested returns the view a provider works through while it loads: the same
// providers, with that provider added to the chain.
func (l *loading) nested(entry *registration) *loading {
	return &loading{
		providers: l.providers,
		load:      l.load,
		ctx:       l.ctx,
		chain:     append(slices.Clip(l.chain), entry),
	}
}

// match returns the deferred providers the abstraction triggers that still need
// loading, in execution order. Providers already loading up the chain are left
// out: one that requests its own types while registering or booting must fall
// through to the container instead of waiting for itself.
func (l *loading) match(abstraction reflect.Type) []*registration {
	var matched []*registration
	for _, entry := range l.providers.list() {
		if !entry.waiting() || slices.Contains(l.chain, entry) || !entry.triggers(abstraction) {
			continue
		}

		matched = append(matched, entry)
	}

	return matched
}

// run loads the given providers in order, depth-first: a provider's own load
// runs to completion — both phases, plus anything it requests while they run —
// before the request that triggered it carries on. Each one is loaded exactly once,
// however many requests trigger it at the same time: its gate runs the load for
// the first caller and holds the others until it finishes, so nobody resolves a
// binding the provider has not registered yet. A provider that fails to load is
// not retried; every request that triggers it reports the error of the one run.
func (l *loading) run(entries []*registration) error {
	for _, entry := range entries {
		if err := entry.gate.do(func() error { return l.load(l.ctx, entry, l.nested(entry)) }); err != nil {
			return err
		}
	}

	return nil
}
