package provider_test

import (
	"context"
	"testing"

	"github.com/danceable/provider/examples/blog/infrastructure/config"
	blogprovider "github.com/danceable/provider/examples/blog/infrastructure/provider"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestConfigProvider(t *testing.T) {
	t.Setenv("BLOG_HTTP_ADDR", ":9999")
	t.Setenv("BLOG_PER_PAGE", "7")

	p := blogprovider.NewConfigProvider()
	assert.Equal(t, 0, p.Order(), "config must register first")

	c := newTestContainer()
	require.NoError(t, p.Register(context.Background(), c))

	var cfg *config.Config
	require.NoError(t, c.Resolve(&cfg))
	assert.Equal(t, ":9999", cfg.HTTPAddr)
	assert.Equal(t, 7, cfg.PerPage)

	require.NoError(t, p.Boot(context.Background(), c))
	require.NoError(t, p.Terminate(context.Background()))
}
