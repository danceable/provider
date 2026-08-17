package provider

import (
	"reflect"
	"slices"

	"github.com/danceable/container/resolve"
)

// trigger is all the deferring container needs of the deferred providers: given
// the types a request is about, load whatever provides them. The container knows
// nothing else about them — not the registry, not the contexts, not the chain.
type trigger interface {
	resolve(abstractions ...reflect.Type) error
}

// deferring decorates a Container so that requesting a type provided by a
// deferred provider loads that provider before the request reaches the
// underlying container. Children created through Scope and Derive keep the
// decoration, so the whole subtree triggers the same deferred providers.
//
// It adds exactly one step to each request and delegates everything else, which
// keeps it a faithful Container: whatever the underlying one does, it still does.
type deferring struct {
	Container

	// trigger loads the providers of the requested types.
	trigger trigger
}

var _ Container = &deferring{}

// newDeferring wraps container so requests against it load the providers of the
// types they are about.
func newDeferring(container Container, trigger trigger) *deferring {
	return &deferring{Container: container, trigger: trigger}
}

// Resolve loads the providers of the requested abstraction, then resolves it.
func (c *deferring) Resolve(abstraction any, opts ...resolve.ResolveOption) error {
	if err := c.trigger.resolve(resolved(abstraction)...); err != nil {
		return err
	}

	return c.Container.Resolve(abstraction, opts...)
}

// Call loads the providers of the receiver's arguments, then calls it.
func (c *deferring) Call(receiver any, opts ...resolve.ResolveOption) error {
	if err := c.trigger.resolve(arguments(receiver)...); err != nil {
		return err
	}

	return c.Container.Call(receiver, opts...)
}

// Fill loads the providers of the injected fields, then fills the structure.
func (c *deferring) Fill(receiver any, opts ...resolve.ResolveOption) error {
	if err := c.trigger.resolve(injected(receiver)...); err != nil {
		return err
	}

	return c.Container.Fill(receiver, opts...)
}

// Scope creates a child scope that keeps triggering the same deferred providers.
func (c *deferring) Scope(name string) Container {
	return newDeferring(c.Container.Scope(name), c.trigger)
}

// Derive creates a child scope that keeps triggering the same deferred providers.
func (c *deferring) Derive() Container {
	return newDeferring(c.Container.Derive(), c.trigger)
}

// The helpers below read the types a request is about. Each mirrors what the
// container itself accepts and returns nothing for a shape the container would
// reject, leaving it to report the error.

// resolved returns the type Resolve is asked for.
func resolved(abstraction any) []reflect.Type {
	t := reflect.TypeOf(abstraction)
	if t == nil || t.Kind() != reflect.Pointer {
		return nil
	}

	return []reflect.Type{t.Elem()}
}

// arguments returns the types Call needs to invoke the receiver.
func arguments(receiver any) []reflect.Type {
	t := reflect.TypeOf(receiver)
	if t == nil || t.Kind() != reflect.Func {
		return nil
	}

	return slices.Collect(t.Ins())
}

// injected returns the types Fill injects into the receiver's tagged fields.
func injected(receiver any) []reflect.Type {
	t := reflect.TypeOf(receiver)
	if t == nil || t.Kind() != reflect.Pointer || t.Elem().Kind() != reflect.Struct {
		return nil
	}

	elem := t.Elem()

	var types []reflect.Type
	for i := range elem.NumField() {
		field := elem.Field(i)
		if _, tagged := field.Tag.Lookup("container"); tagged {
			types = append(types, field.Type)
		}
	}

	return types
}
