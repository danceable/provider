package provider

import (
	"errors"
	"testing"

	"github.com/danceable/container"
	"github.com/danceable/container/bind"
	"github.com/danceable/container/resolve"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Shape is a small abstraction used to exercise the container through the adapter.
type Shape interface {
	Area() int
}

type circle struct {
	area int
}

func (c *circle) Area() int { return c.area }

func TestNewAdapter_ImplementsContainer(t *testing.T) {
	t.Parallel()

	var c Container = newAdapter(container.New())
	require.NotNil(t, c)
}

func TestAdapter_Bind_Resolve(t *testing.T) {
	t.Parallel()

	a := newAdapter(container.New())

	require.NoError(t, a.Bind(func() Shape { return &circle{area: 42} }, bind.Singleton()))

	var s Shape
	require.NoError(t, a.Resolve(&s))
	assert.Equal(t, 42, s.Area())
}

func TestAdapter_Bind_PropagatesError(t *testing.T) {
	t.Parallel()

	a := newAdapter(container.New())

	// A non-function resolver is rejected by the underlying container.
	err := a.Bind("not a function")
	assert.Error(t, err)
}

func TestAdapter_Resolve_PropagatesError(t *testing.T) {
	t.Parallel()

	a := newAdapter(container.New())

	// Nothing is bound, so resolution must fail.
	var s Shape
	assert.Error(t, a.Resolve(&s))
}

func TestAdapter_Bind_WithName(t *testing.T) {
	t.Parallel()

	a := newAdapter(container.New())

	require.NoError(t, a.Bind(func() Shape { return &circle{area: 7} }, bind.WithName("small"), bind.Singleton()))
	require.NoError(t, a.Bind(func() Shape { return &circle{area: 99} }, bind.WithName("big"), bind.Singleton()))

	var small, big Shape
	require.NoError(t, a.Resolve(&small, resolve.WithName("small")))
	require.NoError(t, a.Resolve(&big, resolve.WithName("big")))

	assert.Equal(t, 7, small.Area())
	assert.Equal(t, 99, big.Area())
}

func TestAdapter_Call(t *testing.T) {
	t.Parallel()

	a := newAdapter(container.New())
	require.NoError(t, a.Bind(func() Shape { return &circle{area: 13} }, bind.Singleton()))

	var got int
	require.NoError(t, a.Call(func(s Shape) { got = s.Area() }))
	assert.Equal(t, 13, got)
}

func TestAdapter_Call_PropagatesError(t *testing.T) {
	t.Parallel()

	a := newAdapter(container.New())

	// The receiver depends on an unbound Shape, so Call must return an error.
	err := a.Call(func(Shape) {})
	assert.Error(t, err)
}

func TestAdapter_Fill(t *testing.T) {
	t.Parallel()

	a := newAdapter(container.New())
	require.NoError(t, a.Bind(func() Shape { return &circle{area: 21} }, bind.Singleton()))

	type target struct {
		Shape Shape `container:"type"`
	}

	var dst target
	require.NoError(t, a.Fill(&dst))
	require.NotNil(t, dst.Shape)
	assert.Equal(t, 21, dst.Shape.Area())
}

func TestAdapter_Fill_PropagatesError(t *testing.T) {
	t.Parallel()

	a := newAdapter(container.New())

	// Fill requires a pointer to a struct; a non-pointer is rejected.
	err := a.Fill(struct{}{})
	assert.Error(t, err)
}

func TestAdapter_Reset(t *testing.T) {
	t.Parallel()

	a := newAdapter(container.New())
	require.NoError(t, a.Bind(func() Shape { return &circle{area: 1} }, bind.Singleton()))

	var s Shape
	require.NoError(t, a.Resolve(&s))

	a.Reset()

	// After Reset the binding is gone.
	assert.Error(t, a.Resolve(&s))
}

func TestAdapter_Scope_ReturnsAdapter(t *testing.T) {
	t.Parallel()

	root := container.New()
	a := newAdapter(root)

	scoped := a.Scope("db")

	// Scope must return another adapter, keeping the scope tree behind the interface.
	scopedAdapter, ok := scoped.(*adapter)
	require.True(t, ok)

	// The underlying container is the named child of the root, and Scope is idempotent
	// for a given name, so re-requesting it yields the same concrete container.
	assert.Same(t, root.Scope("db"), scopedAdapter.concrete)
}

func TestAdapter_Scope_InheritsParentBindings(t *testing.T) {
	t.Parallel()

	a := newAdapter(container.New())
	require.NoError(t, a.Bind(func() Shape { return &circle{area: 5} }, bind.Singleton()))

	scoped := a.Scope("request")

	// A child scope resolves bindings registered on an ancestor.
	var s Shape
	require.NoError(t, scoped.Resolve(&s))
	assert.Equal(t, 5, s.Area())
}

func TestAdapter_Scope_BindingsStayLocal(t *testing.T) {
	t.Parallel()

	a := newAdapter(container.New())
	scoped := a.Scope("request")

	require.NoError(t, scoped.Bind(func() Shape { return &circle{area: 8} }, bind.Singleton()))

	// A binding registered on the child must not leak to the parent.
	var s Shape
	assert.Error(t, a.Resolve(&s))

	// ...but it is resolvable from the child itself.
	require.NoError(t, scoped.Resolve(&s))
	assert.Equal(t, 8, s.Area())
}

func TestAdapter_Derive_ReturnsAdapter(t *testing.T) {
	t.Parallel()

	root := container.New()
	a := newAdapter(root)

	derived := a.Derive()

	derivedAdapter, ok := derived.(*adapter)
	require.True(t, ok)

	// A derived scope is an anonymous child whose parent is the root.
	assert.Same(t, root, derivedAdapter.concrete.Parent())

	// Each Derive produces a distinct, unregistered child.
	other := a.Derive().(*adapter)
	assert.NotSame(t, derivedAdapter.concrete, other.concrete)
}

func TestAdapter_Derive_InheritsParentBindings(t *testing.T) {
	t.Parallel()

	a := newAdapter(container.New())
	require.NoError(t, a.Bind(func() Shape { return &circle{area: 3} }, bind.Singleton()))

	derived := a.Derive()

	var s Shape
	require.NoError(t, derived.Resolve(&s))
	assert.Equal(t, 3, s.Area())
}

func TestAdapter_Derive_BindingsStayLocal(t *testing.T) {
	t.Parallel()

	a := newAdapter(container.New())
	derived := a.Derive()

	require.NoError(t, derived.Bind(func() Shape { return &circle{area: 4} }, bind.Singleton()))

	var s Shape
	assert.Error(t, a.Resolve(&s))
}

func TestAdapter_Scope_Nested(t *testing.T) {
	t.Parallel()

	a := newAdapter(container.New())
	require.NoError(t, a.Bind(func() Shape { return &circle{area: 100} }, bind.Singleton()))

	// Scoping is chainable through the interface returned by Scope.
	grandchild := a.Scope("outer").Scope("inner")

	var s Shape
	require.NoError(t, grandchild.Resolve(&s))
	assert.Equal(t, 100, s.Area())
}

// Ensure the error from a failing resolver is surfaced unchanged through the adapter.
func TestAdapter_Bind_ResolverError(t *testing.T) {
	t.Parallel()

	a := newAdapter(container.New())
	sentinel := errors.New("boom")

	require.NoError(t, a.Bind(func() (Shape, error) { return nil, sentinel }, bind.Singleton(), bind.Lazy()))

	var s Shape
	err := a.Resolve(&s)
	assert.ErrorIs(t, err, sentinel)
}
