// Package danceable adapts github.com/danceable/container to the provider's
// backend-agnostic Container contract.
//
// The underlying *container.Container returns its own concrete type from Scope
// and Derive, so it does not satisfy the contract directly. The adapter bridges
// that gap by re-wrapping the children it returns, keeping the whole scope tree
// behind the interface, and by translating the neutral bind/resolve options into
// danceable's own option types.
package danceable

import (
	"github.com/danceable/container"
	"github.com/danceable/container/bind"
	"github.com/danceable/container/resolve"
	"github.com/danceable/provider/internal/contract"
)

// Adapter wraps a *container.Container so it satisfies contract.Container.
type Adapter struct {
	concrete *container.Container
}

var _ contract.Container = (*Adapter)(nil)

// New wraps the given concrete container so it satisfies the Container contract.
func New(concrete *container.Container) *Adapter {
	return &Adapter{concrete: concrete}
}

// Reset clears the underlying container's bindings.
func (a *Adapter) Reset() {
	a.concrete.Reset()
}

// Bind registers a resolver, translating the neutral bind options into
// danceable's own.
func (a *Adapter) Bind(receiver any, opts ...contract.BindOption) error {
	o := contract.ApplyBindOptions(opts...)

	var bopts []bind.BindOption
	if o.Name != "" {
		bopts = append(bopts, bind.WithName(o.Name))
	}
	if o.Singleton {
		bopts = append(bopts, bind.Singleton())
	}
	if o.Lazy {
		bopts = append(bopts, bind.Lazy())
	}

	return a.concrete.Bind(receiver, bopts...)
}

// Call invokes the receiver, injecting its arguments from the container.
func (a *Adapter) Call(receiver any, opts ...contract.ResolveOption) error {
	return a.concrete.Call(receiver, resolveOptions(opts)...)
}

// Resolve resolves an abstraction into the receiver pointer.
func (a *Adapter) Resolve(abstraction any, opts ...contract.ResolveOption) error {
	return a.concrete.Resolve(abstraction, resolveOptions(opts)...)
}

// Fill populates the tagged fields of the receiver struct from the container.
func (a *Adapter) Fill(receiver any, opts ...contract.ResolveOption) error {
	return a.concrete.Fill(receiver, resolveOptions(opts)...)
}

// Scope creates a new named child container for managing scoped dependencies.
func (a *Adapter) Scope(name string) contract.Container {
	return New(a.concrete.Scope(name))
}

// Derive creates a new anonymous child container that inherits the parent's bindings.
func (a *Adapter) Derive() contract.Container {
	return New(a.concrete.Derive())
}

// resolveOptions translates the neutral resolve options into danceable's own.
func resolveOptions(opts []contract.ResolveOption) []resolve.ResolveOption {
	o := contract.ApplyResolveOptions(opts...)

	var ropts []resolve.ResolveOption
	if o.Name != "" {
		ropts = append(ropts, resolve.WithName(o.Name))
	}
	if len(o.Params) > 0 {
		ropts = append(ropts, resolve.WithParams(o.Params...))
	}

	return ropts
}
