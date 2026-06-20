package provider_test

import (
	"context"
	"testing"

	"github.com/danceable/container/bind"
	"github.com/danceable/provider/examples/blog/infrastructure/config"
	blogprovider "github.com/danceable/provider/examples/blog/infrastructure/provider"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestMongoProvider checks the wiring that does not require a running MongoDB:
// the provider registers without error and Terminate is safe before Boot. The
// connection itself (Boot pings the server) is covered by integration tests.
func TestMongoProvider(t *testing.T) {
	t.Parallel()

	p := blogprovider.NewMongoProvider()
	assert.Equal(t, 10, p.Order(), "database comes right after config")

	c := newTestContainer()
	require.NoError(t, c.Bind(func() *config.Config {
		return &config.Config{MongoURI: "mongodb://localhost:27017", MongoDB: "blog"}
	}, bind.Singleton()))

	// The client/database bindings are lazy, so Register succeeds without a server.
	require.NoError(t, p.Register(context.Background(), c))

	// Terminate before Boot has no client to disconnect.
	require.NoError(t, p.Terminate(context.Background()))
}
