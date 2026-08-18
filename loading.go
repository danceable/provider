package provider

import (
	"context"
	"reflect"
	"slices"
)

// loading is a registry's deferred providers, seen from one place in the
// container tree: it loads the ones a request needs.
type loading struct {
	providers *registry

	// base is the container the providers load against, undecorated.
	base Container

	// ctx is the context they are loaded with: their owner's, since a container
	// request carries none.
	ctx context.Context

	// chain is the providers already loading above this view.
	chain []*registration
}

var _ trigger = &loading{}

func newLoading(providers *registry, base Container, ctx context.Context) *loading {
	return &loading{providers: providers, base: base, ctx: ctx}
}

// container decorates the base container, or hands it back untouched when
// nothing is deferred.
func (l *loading) container() Container {
	if !l.any() {
		return l.base
	}

	return newDeferring(l.base, l)
}

func (l *loading) any() bool {
	return l.providers.deferred()
}

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

// nested returns the view a provider works through while it loads.
func (l *loading) nested(entry *registration) *loading {
	return &loading{
		providers: l.providers,
		base:      l.base,
		ctx:       l.ctx,
		chain:     append(slices.Clip(l.chain), entry),
	}
}

// match returns the providers the abstraction triggers that still need loading,
// in execution order. Providers already loading up the chain are left out, so
// one that requests its own types falls through to the container instead of
// waiting for itself.
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

// run loads the providers in order, depth-first: each is completely up before
// the request that triggered it carries on. A gate keeps concurrent requests
// from racing past a load in progress, and a failed load is not retried.
func (l *loading) run(entries []*registration) error {
	for _, entry := range entries {
		err := entry.gate.do(func() error {
			return load(l.ctx, l.providers, entry, newDeferring(l.base, l.nested(entry)))
		})
		if err != nil {
			return err
		}
	}

	return nil
}
