package provider

import (
	"errors"
	"reflect"

	"github.com/danceable/provider/internal/contract"
)

// Container defines the interface for a dependency injection container.
//
// The package never depends on a concrete container: the adapters under
// github.com/danceable/provider/adapters bridge the supported backends,
// deferring decorates any of them with on-demand loading, and an application is
// free to supply its own.
type Container = contract.Container

// The neutral option types, re-exported from the contract so that callers
// configure a container through this package without importing a backend.
type (
	// BindOption configures a binding passed to Container.Bind.
	BindOption = contract.BindOption

	// BindOptions is the resolved configuration of a binding; an adapter reads it
	// to translate the bind onto its backend.
	BindOptions = contract.BindOptions

	// ResolveOption configures a resolution passed to Container.Call, Resolve or Fill.
	ResolveOption = contract.ResolveOption

	// ResolveOptions is the resolved configuration of a resolution.
	ResolveOptions = contract.ResolveOptions
)

// The options themselves, re-exported for the same reason.
var (
	// WithName names a binding, enabling multiple concretes per abstraction.
	WithName = contract.WithName

	// Singleton marks a binding as a single shared instance.
	Singleton = contract.Singleton

	// Lazy defers a resolver until the first resolution.
	Lazy = contract.Lazy

	// ResolveName selects the named binding to resolve.
	ResolveName = contract.WithResolveName

	// WithParams supplies runtime values to satisfy resolver arguments.
	WithParams = contract.WithParams
)

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

	return c.Bind(resolver.Interface(), WithName(name), Singleton())
}
