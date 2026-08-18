// Package contract defines the backend-agnostic dependency-injection contract
// used by the provider manager: the Container interface plus the neutral bind
// and resolve options.
//
// The contract is deliberately free of any concrete container's types so that
// adapters (github.com/danceable/provider/adapters/...) can translate it onto
// different backends. It lives under internal/ purely to break an import cycle:
// the provider package re-exports every name here as provider.Container,
// provider.BindOption, provider.WithName, and so on, so callers never import
// this package directly.
package contract

// Container defines the interface for a dependency injection container. It is a
// neutral surface: each adapter maps these methods and options onto the calls of
// the container it wraps.
type Container interface {
	// Reset deletes all the existing bindings.
	Reset()

	// Bind maps the type a resolver function returns to that resolver.
	Bind(receiver any, opts ...BindOption) error

	// Call invokes a function, resolving its arguments from the container.
	Call(receiver any, opts ...ResolveOption) error

	// Resolve fills a pointer with the concrete bound to the type it points to.
	Resolve(abstraction any, opts ...ResolveOption) error

	// Fill resolves the fields of a struct that carry the container tag.
	Fill(receiver any, opts ...ResolveOption) error

	// Scope returns a named child container, reused on later calls with that name.
	Scope(name string) Container

	// Derive returns an anonymous child container, collected once it is dropped.
	Derive() Container
}

// BindOptions holds the neutral configuration for a single binding. Adapters
// read these fields and translate them into their backend's own bind calls; an
// adapter whose backend lacks a concept (dig, for instance, is always lazy and
// memoized) may ignore the corresponding field.
type BindOptions struct {
	// Name enables multiple concretes per abstraction, disambiguated by name.
	Name string

	// Singleton marks the binding as a single shared instance.
	Singleton bool

	// Lazy defers the resolver invocation until the first resolution.
	Lazy bool
}

// BindOption is a functional option for configuring a binding.
type BindOption func(*BindOptions)

// WithName sets a name for the binding, enabling multiple concretes per abstraction.
func WithName(name string) BindOption {
	return func(o *BindOptions) { o.Name = name }
}

// Singleton marks the binding as a singleton (one shared instance).
func Singleton() BindOption {
	return func(o *BindOptions) { o.Singleton = true }
}

// Lazy defers the resolver invocation until the first time the binding is resolved.
func Lazy() BindOption {
	return func(o *BindOptions) { o.Lazy = true }
}

// ResolveOptions holds the neutral resolve-time parameters.
type ResolveOptions struct {
	// Name selects a named binding to resolve.
	Name string

	// Params supplies runtime values used to satisfy resolver arguments.
	Params []any
}

// ResolveOption is a functional option for resolve-time parameters.
type ResolveOption func(*ResolveOptions)

// WithResolveName selects the named binding to resolve.
func WithResolveName(name string) ResolveOption {
	return func(o *ResolveOptions) { o.Name = name }
}

// WithParams provides runtime values used to satisfy resolver arguments.
func WithParams(params ...any) ResolveOption {
	return func(o *ResolveOptions) { o.Params = append(o.Params, params...) }
}

// ApplyBindOptions collects the given bind options into a BindOptions value.
// Adapters use it to read the neutral configuration before mapping it onto their
// backend.
func ApplyBindOptions(opts ...BindOption) *BindOptions {
	o := &BindOptions{}
	for _, opt := range opts {
		opt(o)
	}
	return o
}

// ApplyResolveOptions collects the given resolve options into a ResolveOptions
// value.
func ApplyResolveOptions(opts ...ResolveOption) *ResolveOptions {
	o := &ResolveOptions{}
	for _, opt := range opts {
		opt(o)
	}
	return o
}
