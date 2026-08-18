package provider

import (
	"reflect"

	"github.com/danceable/container/resolve"
)

// trigger loads whatever provides the given types. It is all the deferring
// container knows about the deferred providers.
type trigger interface {
	resolve(abstractions ...reflect.Type) error
}

// deferring decorates a Container so a request loads the providers of the types
// it is about before reaching the underlying container. Children from Scope and
// Derive keep the decoration.
type deferring struct {
	Container

	trigger trigger
}

var _ Container = &deferring{}

func newDeferring(container Container, trigger trigger) *deferring {
	return &deferring{Container: container, trigger: trigger}
}

func (c *deferring) Resolve(abstraction any, opts ...resolve.ResolveOption) error {
	if err := c.trigger.resolve(resolved(abstraction)...); err != nil {
		return err
	}

	return c.Container.Resolve(abstraction, opts...)
}

func (c *deferring) Call(receiver any, opts ...resolve.ResolveOption) error {
	if err := c.trigger.resolve(arguments(receiver)...); err != nil {
		return err
	}

	return c.Container.Call(receiver, opts...)
}

func (c *deferring) Fill(receiver any, opts ...resolve.ResolveOption) error {
	if err := c.trigger.resolve(injected(receiver)...); err != nil {
		return err
	}

	return c.Container.Fill(receiver, opts...)
}

func (c *deferring) Scope(name string) Container {
	return newDeferring(c.Container.Scope(name), c.trigger)
}

func (c *deferring) Derive() Container {
	return newDeferring(c.Container.Derive(), c.trigger)
}
