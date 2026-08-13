package dig

import (
	"testing"

	"github.com/danceable/provider/internal/contract"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/dig"
)

// Shape is a small abstraction used to exercise the container through the adapter.
type Shape interface {
	Area() int
}

type circle struct {
	area int
}

func (c *circle) Area() int { return c.area }

func TestNew_ImplementsContainer(t *testing.T) {
	t.Parallel()

	var c contract.Container = New(dig.New())
	require.NotNil(t, c)
}

func TestAdapter_Bind_Resolve(t *testing.T) {
	t.Parallel()

	a := New(dig.New())

	require.NoError(t, a.Bind(func() Shape { return &circle{area: 42} }, contract.Singleton()))

	var s Shape
	require.NoError(t, a.Resolve(&s))
	assert.Equal(t, 42, s.Area())
}

func TestAdapter_Bind_PropagatesError(t *testing.T) {
	t.Parallel()

	a := New(dig.New())

	// A non-function constructor is rejected by dig.
	err := a.Bind("not a function")
	assert.Error(t, err)
}

func TestAdapter_Resolve_PropagatesError(t *testing.T) {
	t.Parallel()

	a := New(dig.New())

	// Nothing is provided, so resolution must fail.
	var s Shape
	assert.Error(t, a.Resolve(&s))
}

func TestAdapter_Resolve_RejectsNonPointer(t *testing.T) {
	t.Parallel()

	a := New(dig.New())

	var s Shape
	assert.ErrorIs(t, a.Resolve(s), ErrInvalidReceiver)
}

func TestAdapter_Bind_WithName(t *testing.T) {
	t.Parallel()

	a := New(dig.New())

	require.NoError(t, a.Bind(func() Shape { return &circle{area: 7} }, contract.WithName("small")))
	require.NoError(t, a.Bind(func() Shape { return &circle{area: 99} }, contract.WithName("big")))

	var small, big Shape
	require.NoError(t, a.Resolve(&small, contract.WithResolveName("small")))
	require.NoError(t, a.Resolve(&big, contract.WithResolveName("big")))

	assert.Equal(t, 7, small.Area())
	assert.Equal(t, 99, big.Area())
}

func TestAdapter_Call(t *testing.T) {
	t.Parallel()

	a := New(dig.New())
	require.NoError(t, a.Bind(func() Shape { return &circle{area: 13} }))

	var got int
	require.NoError(t, a.Call(func(s Shape) { got = s.Area() }))
	assert.Equal(t, 13, got)
}

func TestAdapter_Call_PropagatesError(t *testing.T) {
	t.Parallel()

	a := New(dig.New())

	// The receiver depends on an unprovided Shape, so Call must return an error.
	err := a.Call(func(Shape) {})
	assert.Error(t, err)
}

func TestAdapter_Fill_ByType(t *testing.T) {
	t.Parallel()

	a := New(dig.New())
	require.NoError(t, a.Bind(func() Shape { return &circle{area: 21} }))

	type target struct {
		Shape Shape `container:"type"`
	}

	var dst target
	require.NoError(t, a.Fill(&dst))
	require.NotNil(t, dst.Shape)
	assert.Equal(t, 21, dst.Shape.Area())
}

func TestAdapter_Fill_ByName(t *testing.T) {
	t.Parallel()

	a := New(dig.New())
	require.NoError(t, a.Bind(func() Shape { return &circle{area: 55} }, contract.WithName("Shape")))

	type target struct {
		Shape Shape `container:"name"`
	}

	var dst target
	require.NoError(t, a.Fill(&dst))
	require.NotNil(t, dst.Shape)
	assert.Equal(t, 55, dst.Shape.Area())
}

func TestAdapter_Fill_RejectsNonStructPointer(t *testing.T) {
	t.Parallel()

	a := New(dig.New())

	assert.ErrorIs(t, a.Fill(struct{}{}), ErrInvalidReceiver)
}

func TestAdapter_Fill_InvalidTag(t *testing.T) {
	t.Parallel()

	a := New(dig.New())

	type target struct {
		Shape Shape `container:"bogus"`
	}

	var dst target
	assert.ErrorIs(t, a.Fill(&dst), ErrInvalidStructTag)
}

func TestAdapter_Reset(t *testing.T) {
	t.Parallel()

	a := New(dig.New())
	require.NoError(t, a.Bind(func() Shape { return &circle{area: 1} }))

	var s Shape
	require.NoError(t, a.Resolve(&s))

	a.Reset()

	// After Reset the root container is rebuilt, so the binding is gone.
	assert.Error(t, a.Resolve(&s))
}

func TestAdapter_Scope_ReturnsAdapter(t *testing.T) {
	t.Parallel()

	a := New(dig.New())

	scoped := a.Scope("db")

	// Scope must return another adapter, keeping the scope tree behind the interface.
	_, ok := scoped.(*Adapter)
	require.True(t, ok)
}

func TestAdapter_Scope_InheritsParentBindings(t *testing.T) {
	t.Parallel()

	a := New(dig.New())
	require.NoError(t, a.Bind(func() Shape { return &circle{area: 5} }))

	scoped := a.Scope("request")

	// A child scope resolves bindings registered on an ancestor.
	var s Shape
	require.NoError(t, scoped.Resolve(&s))
	assert.Equal(t, 5, s.Area())
}

func TestAdapter_Scope_BindingsStayLocal(t *testing.T) {
	t.Parallel()

	a := New(dig.New())
	scoped := a.Scope("request")

	require.NoError(t, scoped.Bind(func() Shape { return &circle{area: 8} }))

	// A binding registered on the child must not leak to the parent.
	var s Shape
	assert.Error(t, a.Resolve(&s))

	// ...but it is resolvable from the child itself.
	require.NoError(t, scoped.Resolve(&s))
	assert.Equal(t, 8, s.Area())
}

func TestAdapter_Derive_ReturnsAdapter(t *testing.T) {
	t.Parallel()

	a := New(dig.New())

	derived := a.Derive()

	_, ok := derived.(*Adapter)
	require.True(t, ok)

	// Each Derive produces a distinct child scope.
	other := a.Derive()
	assert.NotSame(t, derived, other)
}

func TestAdapter_Derive_InheritsParentBindings(t *testing.T) {
	t.Parallel()

	a := New(dig.New())
	require.NoError(t, a.Bind(func() Shape { return &circle{area: 3} }))

	derived := a.Derive()

	var s Shape
	require.NoError(t, derived.Resolve(&s))
	assert.Equal(t, 3, s.Area())
}

func TestAdapter_Derive_BindingsStayLocal(t *testing.T) {
	t.Parallel()

	a := New(dig.New())
	derived := a.Derive()

	require.NoError(t, derived.Bind(func() Shape { return &circle{area: 4} }))

	var s Shape
	assert.Error(t, a.Resolve(&s))
}

func TestAdapter_Scope_Nested(t *testing.T) {
	t.Parallel()

	a := New(dig.New())
	require.NoError(t, a.Bind(func() Shape { return &circle{area: 100} }))

	// Scoping is chainable through the interface returned by Scope.
	grandchild := a.Scope("outer").Scope("inner")

	var s Shape
	require.NoError(t, grandchild.Resolve(&s))
	assert.Equal(t, 100, s.Area())
}
