package provider_test

import (
	"context"
	"testing"
	"time"

	"github.com/danceable/provider"
	"github.com/stretchr/testify/assert"
)

func TestDefaultOptions(t *testing.T) {
	t.Parallel()

	opts := provider.DefaultOptions()

	assert.Equal(t, 300*time.Millisecond, opts.TerminationDelay)
	assert.Equal(t, 200*time.Millisecond, opts.TerminationDeadline)
	assert.Nil(t, opts.Callback)
}

func TestWithTerminationDelay(t *testing.T) {
	t.Parallel()

	m := provider.New(new(containerMock))

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	err := m.Run(ctx, provider.WithTerminationDelay(10*time.Millisecond))
	assert.NoError(t, err)
}

func TestWithTerminationDeadline(t *testing.T) {
	t.Parallel()

	m := provider.New(new(containerMock))

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	err := m.Run(ctx, provider.WithTerminationDelay(0), provider.WithTerminationDeadline(15*time.Second))
	assert.NoError(t, err)
}

func TestWithCallback(t *testing.T) {
	t.Parallel()

	called := false
	cb := func(_ context.Context, _ provider.Container) {
		called = true
	}

	m := provider.New(new(containerMock))

	ctx, cancel := context.WithCancel(context.Background())

	err := m.Run(ctx, provider.WithTerminationDelay(0), provider.WithCallback(func(ctx context.Context, c provider.Container) {
		cb(ctx, c)
		cancel()
	}))
	assert.NoError(t, err)
	assert.True(t, called)
}

func TestMultipleOptions(t *testing.T) {
	t.Parallel()

	called := false

	m := provider.New(new(containerMock))

	ctx, cancel := context.WithCancel(context.Background())

	err := m.Run(ctx,
		provider.WithTerminationDelay(1*time.Millisecond),
		provider.WithTerminationDeadline(2*time.Second),
		provider.WithCallback(func(ctx context.Context, c provider.Container) {
			called = true
			cancel()
		}),
	)
	assert.NoError(t, err)
	assert.True(t, called)
}
