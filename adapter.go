package provider

import (
	"github.com/danceable/container"
	"github.com/danceable/container/bind"
	"github.com/danceable/container/resolve"
)

// adapter implements the Container interface by delegating calls to the underlying container.
//
// The underlying *container.Container returns its own concrete type from Scope and Derive,
// so it does not satisfy the Container interface directly. The adapter bridges that gap by
// re-wrapping the children it returns, keeping the whole scope tree behind the interface.
type adapter struct {
	concrete *container.Container
}

var _ Container = &adapter{}

// newAdapter wraps the given concrete container so it satisfies the Container interface.
func newAdapter(concrete *container.Container) *adapter {
	return &adapter{concrete: concrete}
}

// Reset calls the same method of the default concrete.
func (a *adapter) Reset() {
	a.concrete.Reset()
}

// Bind calls the same method of the default concrete.
func (a *adapter) Bind(receiver any, opts ...bind.BindOption) error {
	return a.concrete.Bind(receiver, opts...)
}

// Call calls the same method of the default concrete.
func (a *adapter) Call(receiver any, opts ...resolve.ResolveOption) error {
	return a.concrete.Call(receiver, opts...)
}

// Resolve calls the same method of the default concrete.
func (a *adapter) Resolve(abstraction any, opts ...resolve.ResolveOption) error {
	return a.concrete.Resolve(abstraction, opts...)
}

// Fill calls the same method of the default concrete.
func (a *adapter) Fill(receiver any, opts ...resolve.ResolveOption) error {
	return a.concrete.Fill(receiver, opts...)
}

// Scope creates a new child container with the given name, which can be used to manage scoped dependencies.
func (a *adapter) Scope(name string) Container {
	return newAdapter(a.concrete.Scope(name))
}

// Derive creates a new child container that inherits the binding of the parent container.
func (a *adapter) Derive() Container {
	return newAdapter(a.concrete.Derive())
}
