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
	// Reset deletes all the existing bindings.
	Reset()

	// Bind maps the type a resolver function returns to that resolver.
	Bind(receiver any, opts ...bind.BindOption) error

	// Call invokes a function, resolving its arguments from the container.
	Call(receiver any, opts ...resolve.ResolveOption) error

	// Resolve fills a pointer with the concrete bound to the type it points to.
	Resolve(abstraction any, opts ...resolve.ResolveOption) error

	// Fill resolves the fields of a struct that carry the container tag.
	Fill(receiver any, opts ...resolve.ResolveOption) error

	// Scope returns a named child container, reused on later calls with that name.
	Scope(name string) Container

	// Derive returns an anonymous child container, collected once it is dropped.
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
