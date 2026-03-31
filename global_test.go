package provider_test

import (
	"context"
	"testing"

	"github.com/danceable/provider"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGlobal_Register(t *testing.T) {
	t.Parallel()

	original := provider.Default
	defer func() { provider.Default = original }()

	provider.Default = provider.New(new(containerMock))

	p := new(providerMock)
	setupProviderMock(p, "g", nil, nil, nil, nil)
	provider.Register(p)

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	require.NoError(t, provider.Run(ctx))
	p.AssertExpectations(t)
}

func TestGlobal_Run(t *testing.T) {
	t.Parallel()

	original := provider.Default
	defer func() { provider.Default = original }()

	var calls []string
	provider.Default = provider.New(new(containerMock))

	p := new(providerMock)
	setupProviderMock(p, "g", &calls, nil, nil, nil)
	provider.Register(p)

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	err := provider.Run(ctx)
	require.NoError(t, err)

	assert.Equal(t, []string{"g.Register", "g.Boot", "g.Terminate"}, calls)
}
