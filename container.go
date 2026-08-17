package provider

import (
	"errors"
	"reflect"

	"github.com/danceable/container/bind"
	"github.com/danceable/container/resolve"
)

// Container defines the interface for a dependency injection container.
//
// The package never depends on a concrete container: adapter bridges the
// reference implementation, deferring decorates any of them with on-demand
// loading, and an application is free to supply its own.
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

	// Scope creates a new child container with the given name, which can be used to manage scoped dependencies.
	Scope(name string) Container

	// Derive creates a new child container that inherits the binding of the parent container.
	Derive() Container
}

// ErrNilScopeValue is returned when a nil value is passed to WithValue. The
// container binds values by their reflected type, which cannot be determined
// from an untyped nil.
var ErrNilScopeValue = errors.New("provider: scope value must not be nil")

// bindValue binds value into the container as a named singleton. The container
// only accepts function resolvers, so the value is wrapped in a generated
// func() T returning it.
func bindValue(c Container, name string, value any) error {
	v := reflect.ValueOf(value)
	if !v.IsValid() {
		return ErrNilScopeValue
	}

	resolver := reflect.MakeFunc(
		reflect.FuncOf(nil, []reflect.Type{v.Type()}, false),
		func([]reflect.Value) []reflect.Value { return []reflect.Value{v} },
	)

	return c.Bind(resolver.Interface(), bind.WithName(name), bind.Singleton())
}
