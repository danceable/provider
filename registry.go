package provider

import (
	"cmp"
	"reflect"
	"slices"
	"sync"
	"sync/atomic"
)

// registration is the record a registry keeps for one provider: where it runs in
// the order, how far it got, and — for a deferred provider — the types whose
// request triggers it and the gate that loads it exactly once.
type registration struct {
	// provider is the registered provider.
	provider Provider

	// order is the execution order of the provider. Lower runs first.
	order int

	// types are the types a deferred provider binds, as reported by Provides.
	// It is empty for a provider that runs at boot.
	types []reflect.Type

	// registered reports whether the provider's Register has run. It is the
	// condition for terminating the provider: one that got that far may hold
	// resources, whether or not it went on to boot.
	registered atomic.Bool

	// booted reports whether the provider is fully up. For a deferred provider
	// it is also what stops further requests from loading it again.
	booted atomic.Bool

	// gate loads a deferred provider once, however many requests trigger it at
	// the same time. It is nil for a provider that runs at boot.
	gate *gate
}

// newRegistration creates the record of a provider. A non-empty types defers it
// until one of those types is requested.
func newRegistration(provider Provider, order int, types []reflect.Type) *registration {
	entry := &registration{provider: provider, order: order, types: types}
	if entry.deferred() {
		entry.gate = newGate()
	}

	return entry
}

// deferred reports whether the provider waits for one of its types to be
// requested instead of running at boot.
func (r *registration) deferred() bool {
	return len(r.types) > 0
}

// waiting reports whether the provider is deferred and is not up yet.
func (r *registration) waiting() bool {
	return r.deferred() && !r.booted.Load()
}

// triggers reports whether requesting abstraction should load the provider.
// A provided type triggers on an exact match and, mirroring the container's own
// interface-implementation fallback, on any interface it implements.
func (r *registration) triggers(abstraction reflect.Type) bool {
	for _, provided := range r.types {
		if provided == abstraction {
			return true
		}

		if abstraction.Kind() == reflect.Interface && provided.Implements(abstraction) {
			return true
		}
	}

	return false
}

// registry is the ordered set of registered providers. The manager keeps one for
// the providers it runs itself and one holding the scoped providers, which every
// scope copies. Deferred providers sit in it like any other; what sets them apart
// is only when they run.
type registry struct {
	// entries holds the registrations, lowest order first and, within one order,
	// in registration order.
	entries []*registration

	// waiting counts the deferred providers that have not run yet, so asking
	// whether anything is still deferred costs a single atomic read.
	waiting atomic.Int32

	// mu guards entries.
	mu sync.RWMutex
}

// newRegistry creates an empty registry.
func newRegistry() *registry {
	return &registry{}
}

// add registers a provider. A non-empty types defers it until one of those types
// is requested.
func (r *registry) add(provider Provider, types []reflect.Type) {
	r.mu.Lock()
	defer r.mu.Unlock()

	entry := newRegistration(provider, r.slot(provider), types)

	r.entries = append(r.entries, entry)
	slices.SortStableFunc(r.entries, func(a, b *registration) int { return cmp.Compare(a.order, b.order) })

	if entry.deferred() {
		r.waiting.Add(1)
	}
}

// slot returns the execution order of the provider: the one it declares, or the
// next free slot for providers that declare none. It must be called with the
// lock held.
func (r *registry) slot(provider Provider) int {
	if declared, ok := order(provider); ok {
		return declared
	}

	slots := 0
	for i, entry := range r.entries {
		if i == 0 || entry.order != r.entries[i-1].order {
			slots++
		}
	}

	return slots
}

// list returns the registrations in execution order.
func (r *registry) list() []*registration {
	r.mu.RLock()
	defer r.mu.RUnlock()

	return slices.Clone(r.entries)
}

// clone returns a copy of the registry. The providers are shared, how far they
// got is not: the copy starts fresh, so a scope registers, boots and terminates
// the same providers on its own.
func (r *registry) clone() *registry {
	r.mu.RLock()
	defer r.mu.RUnlock()

	clone := &registry{entries: make([]*registration, len(r.entries))}

	deferred := 0
	for i, entry := range r.entries {
		clone.entries[i] = newRegistration(entry.provider, entry.order, entry.types)
		if entry.deferred() {
			deferred++
		}
	}
	clone.waiting.Store(int32(deferred))

	return clone
}

// deferred reports whether at least one deferred provider is still waiting to be
// loaded.
func (r *registry) deferred() bool {
	return r.waiting.Load() > 0
}

// booted records that the provider is up, so nothing waits for it any more.
func (r *registry) booted(entry *registration) {
	entry.booted.Store(true)

	if entry.deferred() {
		r.waiting.Add(-1)
	}
}
