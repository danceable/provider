// Package dig adapts go.uber.org/dig to the provider's backend-agnostic
// Container contract.
//
// dig models a container as a tree of scopes: *dig.Container is the root scope
// and *dig.Scope its children, both exposing Provide/Invoke/Scope. The adapter
// wraps either behind the digScope interface and re-wraps the children it
// returns, keeping the whole scope tree behind the contract.
//
// dig's model differs from the contract in a few places; the adapter bridges the
// gap and documents the semantics:
//
//   - Bindings are always lazy and memoized per scope, so the neutral Singleton
//     and Lazy options are accepted but have no additional effect.
//   - dig has no runtime resolve parameters, so ResolveOptions.Params is ignored.
//   - dig cannot clear an existing scope; Reset rebuilds the root container and
//     is a no-op on child scopes.
package dig

import (
	"errors"
	"fmt"
	"reflect"
	"sync/atomic"
	"unsafe"

	"github.com/danceable/provider/internal/contract"
	"go.uber.org/dig"
)

// digScope is the common surface of *dig.Container and *dig.Scope that the
// adapter drives. Both the root container and its child scopes satisfy it.
type digScope interface {
	Provide(constructor any, opts ...dig.ProvideOption) error
	Invoke(function any, opts ...dig.InvokeOption) error
	Scope(name string, opts ...dig.ScopeOption) *dig.Scope
}

// ErrInvalidReceiver is returned when Resolve or Fill is given something other
// than the pointer (Resolve) or pointer-to-struct (Fill) they require.
var ErrInvalidReceiver = errors.New("provider/dig: receiver must be a pointer")

// ErrInvalidStructTag is returned when Fill encounters a container struct tag
// whose value is neither "type" nor "name".
var ErrInvalidStructTag = errors.New(`provider/dig: container struct tag must be "type" or "name"`)

// deriveCounter names the anonymous scopes produced by Derive; dig requires
// every scope to have a name.
var deriveCounter atomic.Uint64

// Adapter wraps a dig scope so it satisfies contract.Container.
type Adapter struct {
	scope digScope

	// root marks the adapter that wraps the *dig.Container, the only scope that
	// Reset can rebuild.
	root bool
}

var _ contract.Container = (*Adapter)(nil)

// New wraps the given dig container so it satisfies the Container contract.
func New(container *dig.Container) *Adapter {
	return &Adapter{scope: container, root: true}
}

// newChild wraps a child dig scope produced by Scope or Derive.
func newChild(scope *dig.Scope) *Adapter {
	return &Adapter{scope: scope}
}

// Reset rebuilds the root container, dropping its bindings. dig cannot clear a
// child scope, so Reset is a no-op on scopes created by Scope or Derive.
func (a *Adapter) Reset() {
	if a.root {
		a.scope = dig.New()
	}
}

// Bind registers a constructor with dig. Only the neutral Name option maps onto
// dig (via dig.Name); dig bindings are inherently lazy and memoized, so
// Singleton and Lazy carry no additional meaning here.
func (a *Adapter) Bind(receiver any, opts ...contract.BindOption) error {
	o := contract.ApplyBindOptions(opts...)

	var popts []dig.ProvideOption
	if o.Name != "" {
		popts = append(popts, dig.Name(o.Name))
	}

	return a.scope.Provide(receiver, popts...)
}

// Call invokes the receiver, injecting its arguments from the container. dig
// injects strictly by type, so resolve options do not apply.
func (a *Adapter) Call(receiver any, _ ...contract.ResolveOption) error {
	return a.scope.Invoke(receiver)
}

// Resolve resolves an abstraction into the receiver pointer, honouring the
// neutral Name option. Params are not supported by dig and are ignored.
func (a *Adapter) Resolve(abstraction any, opts ...contract.ResolveOption) error {
	o := contract.ApplyResolveOptions(opts...)
	return a.resolveInto(abstraction, o.Name)
}

// Fill populates the container-tagged fields of the receiver struct, mirroring
// the danceable adapter's tag semantics: `container:"type"` resolves by type
// (using the option Name, if any) and `container:"name"` resolves by the field's
// name.
func (a *Adapter) Fill(receiver any, opts ...contract.ResolveOption) error {
	o := contract.ApplyResolveOptions(opts...)

	rv := reflect.ValueOf(receiver)
	if rv.Kind() != reflect.Pointer || rv.Elem().Kind() != reflect.Struct {
		return ErrInvalidReceiver
	}

	s := rv.Elem()
	st := s.Type()
	for i := range st.NumField() {
		tag, ok := st.Field(i).Tag.Lookup("container")
		if !ok {
			continue
		}

		var name string
		switch tag {
		case "type":
			name = o.Name
		case "name":
			name = st.Field(i).Name
		default:
			return fmt.Errorf("%w; the field is: %s", ErrInvalidStructTag, st.Field(i).Name)
		}

		// Address the field even when unexported, matching danceable's Fill.
		f := s.Field(i)
		ptr := reflect.NewAt(f.Type(), unsafe.Pointer(f.UnsafeAddr()))
		if err := a.resolveInto(ptr.Interface(), name); err != nil {
			return err
		}
	}

	return nil
}

// Scope creates a new named child scope.
func (a *Adapter) Scope(name string) contract.Container {
	return newChild(a.scope.Scope(name))
}

// Derive creates a new anonymous child scope. dig requires a name, so a unique
// one is generated for each derived scope.
func (a *Adapter) Derive() contract.Container {
	name := fmt.Sprintf("derived-%d", deriveCounter.Add(1))
	return newChild(a.scope.Scope(name))
}

// resolveInto resolves a value of the receiver pointer's element type out of the
// scope and assigns it. dig has no direct "resolve into pointer" call, so a
// one-argument function is synthesised and invoked: dig injects the argument,
// the function stores it. A non-empty name resolves a named value via a
// generated dig.In parameter struct.
func (a *Adapter) resolveInto(abstraction any, name string) error {
	rv := reflect.ValueOf(abstraction)
	if rv.Kind() != reflect.Pointer {
		return ErrInvalidReceiver
	}

	target := rv.Elem()
	typ := target.Type()

	if name == "" {
		fnType := reflect.FuncOf([]reflect.Type{typ}, nil, false)
		fn := reflect.MakeFunc(fnType, func(args []reflect.Value) []reflect.Value {
			target.Set(args[0])
			return nil
		})

		return a.scope.Invoke(fn.Interface())
	}

	// Named values are injected through a dig.In struct with a `name`-tagged
	// field; build one dynamically around the target type.
	inType := reflect.StructOf([]reflect.StructField{
		{Name: "In", Anonymous: true, Type: reflect.TypeOf(dig.In{})},
		{Name: "Field", Type: typ, Tag: reflect.StructTag(fmt.Sprintf(`name:%q`, name))},
	})
	fnType := reflect.FuncOf([]reflect.Type{inType}, nil, false)
	fn := reflect.MakeFunc(fnType, func(args []reflect.Value) []reflect.Value {
		target.Set(args[0].Field(1))
		return nil
	})

	return a.scope.Invoke(fn.Interface())
}
