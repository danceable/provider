package contract_test

import (
	"testing"

	"github.com/danceable/provider/internal/contract"
	"github.com/stretchr/testify/assert"
)

// The contract carries no behaviour beyond collecting options, so what is worth
// pinning down is what an adapter reads back: the zero configuration, the effect
// of each option, and how repeated options combine.

func TestApplyBindOptions(t *testing.T) {
	t.Parallel()

	t.Run("no options leave the zero configuration", func(t *testing.T) {
		t.Parallel()

		assert.Equal(t, &contract.BindOptions{}, contract.ApplyBindOptions())
	})

	t.Run("every option is applied", func(t *testing.T) {
		t.Parallel()

		options := contract.ApplyBindOptions(
			contract.WithName("cache"),
			contract.Singleton(),
			contract.Lazy(),
		)

		assert.Equal(t, &contract.BindOptions{Name: "cache", Singleton: true, Lazy: true}, options)
	})

	t.Run("each option is independent", func(t *testing.T) {
		t.Parallel()

		// An adapter branches on each field on its own, so one option must not
		// set another.
		assert.Equal(t, &contract.BindOptions{Name: "cache"}, contract.ApplyBindOptions(contract.WithName("cache")))
		assert.Equal(t, &contract.BindOptions{Singleton: true}, contract.ApplyBindOptions(contract.Singleton()))
		assert.Equal(t, &contract.BindOptions{Lazy: true}, contract.ApplyBindOptions(contract.Lazy()))
	})

	t.Run("the last name wins", func(t *testing.T) {
		t.Parallel()

		options := contract.ApplyBindOptions(contract.WithName("first"), contract.WithName("second"))

		assert.Equal(t, "second", options.Name)
	})
}

func TestApplyResolveOptions(t *testing.T) {
	t.Parallel()

	t.Run("no options leave the zero configuration", func(t *testing.T) {
		t.Parallel()

		assert.Equal(t, &contract.ResolveOptions{}, contract.ApplyResolveOptions())
	})

	t.Run("every option is applied", func(t *testing.T) {
		t.Parallel()

		options := contract.ApplyResolveOptions(
			contract.WithResolveName("primary"),
			contract.WithParams(5, "second"),
		)

		assert.Equal(t, &contract.ResolveOptions{Name: "primary", Params: []any{5, "second"}}, options)
	})

	t.Run("the last name wins", func(t *testing.T) {
		t.Parallel()

		options := contract.ApplyResolveOptions(
			contract.WithResolveName("first"),
			contract.WithResolveName("second"),
		)

		assert.Equal(t, "second", options.Name)
	})

	t.Run("params accumulate in order", func(t *testing.T) {
		t.Parallel()

		// Unlike the scalar fields, params append: an adapter hands the whole
		// slice to its backend, and the order is what positional matching uses.
		options := contract.ApplyResolveOptions(contract.WithParams(1), contract.WithParams(2, 3))

		assert.Equal(t, []any{1, 2, 3}, options.Params)
	})

	t.Run("no params leave the slice nil", func(t *testing.T) {
		t.Parallel()

		// The adapter tests len() to decide whether to pass params on at all.
		assert.Nil(t, contract.ApplyResolveOptions(contract.WithParams()).Params)
	})
}
