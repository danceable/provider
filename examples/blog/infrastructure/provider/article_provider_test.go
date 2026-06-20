package provider_test

import (
	"context"
	"testing"

	"github.com/danceable/container/bind"
	app "github.com/danceable/provider/examples/blog/application/article"
	domain "github.com/danceable/provider/examples/blog/domain/article"
	"github.com/danceable/provider/examples/blog/infrastructure/mongodb"
	blogprovider "github.com/danceable/provider/examples/blog/infrastructure/provider"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.mongodb.org/mongo-driver/v2/mongo"
)

// TestArticleProvider proves the dependency-inversion seam: given a
// *mongo.Database in the container, the provider binds the domain.Repository
// port and the application service on top of it. The database is created but
// never dialed (mongo.Connect is lazy), which is enough because the bindings
// are lazy and no test resolves data from MongoDB.
func TestArticleProvider(t *testing.T) {
	t.Parallel()

	p := blogprovider.NewArticleProvider()
	assert.Equal(t, 20, p.Order(), "domain wiring comes after the database")

	client, err := mongodb.Connect("mongodb://localhost:27017")
	require.NoError(t, err)
	db := client.Database("blog")

	c := newTestContainer()
	require.NoError(t, c.Bind(func() *mongo.Database { return db }, bind.Singleton(), bind.Lazy()))
	require.NoError(t, p.Register(context.Background(), c))

	var repo domain.Repository
	require.NoError(t, c.Resolve(&repo))
	assert.NotNil(t, repo, "the repository port is bound")

	var svc *app.Service
	require.NoError(t, c.Resolve(&svc))
	assert.NotNil(t, svc, "the service is bound on top of the port")

	require.NoError(t, p.Boot(context.Background(), c))
	require.NoError(t, p.Terminate(context.Background()))
}
