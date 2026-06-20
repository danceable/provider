package provider_test

import (
	"context"
	"testing"

	"github.com/danceable/provider/examples/blog/infrastructure/i18n"
	blogprovider "github.com/danceable/provider/examples/blog/infrastructure/provider"
	"github.com/danceable/provider/examples/blog/presenation/http/middlewares"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestI18nProvider checks the global i18n wiring: it binds the translation
// repository and the request-scope opener that the HTTP middleware relies on.
func TestI18nProvider(t *testing.T) {
	t.Parallel()

	p := blogprovider.NewI18nProvider()
	assert.Equal(t, 5, p.Order(), "i18n registers after config but before HTTP")

	c := newTestContainer()
	require.NoError(t, p.Register(context.Background(), c))

	var repo i18n.Repository
	require.NoError(t, c.Resolve(&repo))
	got, ok := repo.Translate(i18n.English, "nav.home")
	assert.True(t, ok)
	assert.Equal(t, "Home", got)

	var scoper middlewares.Scoper
	require.NoError(t, c.Resolve(&scoper))
	assert.NotNil(t, scoper, "the HTTP middleware's scope opener is bound")

	require.NoError(t, p.Boot(context.Background(), c))
	require.NoError(t, p.Terminate(context.Background()))
}
