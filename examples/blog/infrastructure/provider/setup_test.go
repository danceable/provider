package provider_test

import (
	"github.com/danceable/container"
	"github.com/danceable/container/bind"
	"github.com/danceable/container/resolve"
	"github.com/danceable/provider"
)

// testContainer wraps a real *container.Container so it satisfies the
// provider.Container interface. The production adapter that does this lives
// unexported inside the provider package, so the tests reproduce the same thin
// delegation here to exercise providers against a genuine container.
type testContainer struct {
	concrete *container.Container
}

var _ provider.Container = (*testContainer)(nil)

// newTestContainer returns an isolated container for a single test.
func newTestContainer() *testContainer {
	return &testContainer{concrete: container.New()}
}

func (c *testContainer) Reset() { c.concrete.Reset() }

func (c *testContainer) Bind(receiver any, opts ...bind.BindOption) error {
	return c.concrete.Bind(receiver, opts...)
}

func (c *testContainer) Call(receiver any, opts ...resolve.ResolveOption) error {
	return c.concrete.Call(receiver, opts...)
}

func (c *testContainer) Resolve(abstraction any, opts ...resolve.ResolveOption) error {
	return c.concrete.Resolve(abstraction, opts...)
}

func (c *testContainer) Fill(receiver any, opts ...resolve.ResolveOption) error {
	return c.concrete.Fill(receiver, opts...)
}

func (c *testContainer) Scope(name string) provider.Container {
	return &testContainer{concrete: c.concrete.Scope(name)}
}

func (c *testContainer) Derive() provider.Container {
	return &testContainer{concrete: c.concrete.Derive()}
}
