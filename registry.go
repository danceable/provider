package provider

import (
	"cmp"
	"reflect"
	"slices"
	"sync"
	"sync/atomic"
)

// registration is what a registry keeps for one provider: where it runs in the
// order, how far it got, and — for a deferred provider — the types that trigger
// it and the gate that loads it once.
type registration struct {
	provider Provider
	order    int

	// types are what a deferred provider binds; empty for one that runs at boot.
	types []reflect.Type

	// registered is the condition for terminating the provider: getting that far
	// may already have acquired something.
	registered atomic.Bool
	booted     atomic.Bool

	// gate is nil for a provider that runs at boot.
	gate *gate
}

func newRegistration(provider Provider, order int, types []reflect.Type) *registration {
	entry := &registration{provider: provider, order: order, types: types}
	if entry.deferred() {
		entry.gate = newGate()
	}

	return entry
}

func (r *registration) deferred() bool {
	return len(r.types) > 0
}

func (r *registration) waiting() bool {
	return r.deferred() && !r.booted.Load()
}

// triggers reports whether requesting abstraction should load the provider: an
// exact match, or an interface a provided type implements, mirroring the
// container's own lookup fallback.
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
// scope copies. Deferred providers sit in it like any other.
type registry struct {
	// entries are ordered lowest first and, within one order, by registration.
	entries []*registration

	// waiting counts the deferred providers that have not run yet.
	waiting atomic.Int32

	mu sync.RWMutex
}

func newRegistry() *registry {
	return &registry{}
}

// add registers a provider. A non-empty types defers it until one of them is
// requested.
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

// slot returns the order the provider declares, or the next free one. It must be
// called with the lock held.
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

func (r *registry) list() []*registration {
	r.mu.RLock()
	defer r.mu.RUnlock()

	return slices.Clone(r.entries)
}

// clone copies the registry for a scope: the providers are shared, how far they
// got is not.
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

func (r *registry) deferred() bool {
	return r.waiting.Load() > 0
}

func (r *registry) booted(entry *registration) {
	entry.booted.Store(true)

	if entry.deferred() {
		r.waiting.Add(-1)
	}
}
