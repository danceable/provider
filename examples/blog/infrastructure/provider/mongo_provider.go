package provider

import (
	"context"
	"time"

	"github.com/danceable/container/bind"
	"github.com/danceable/provider"
	"github.com/danceable/provider/examples/blog/infrastructure/config"
	"github.com/danceable/provider/examples/blog/infrastructure/mongodb"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/readpref"
)

// MongoProvider connects to MongoDB and exposes the client and database to the
// rest of the container. It owns the connection lifecycle: it verifies the
// connection on Boot and closes it on Terminate.
type MongoProvider struct {
	client *mongo.Client
}

var _ provider.Provider = (*MongoProvider)(nil)

// NewMongoProvider creates a MongoProvider.
func NewMongoProvider() *MongoProvider { return &MongoProvider{} }

// Order places the database right after configuration.
func (p *MongoProvider) Order() int { return 10 }

// Register binds the Mongo client and database. Both are lazy singletons so the
// actual connection is established when first resolved (during Boot).
func (p *MongoProvider) Register(_ context.Context, c provider.Container) error {
	if err := c.Bind(func(cfg *config.Config) (*mongo.Client, error) {
		return mongodb.Connect(cfg.MongoURI)
	}, bind.Singleton(), bind.Lazy()); err != nil {
		return err
	}

	return c.Bind(func(client *mongo.Client, cfg *config.Config) *mongo.Database {
		return client.Database(cfg.MongoDB)
	}, bind.Singleton(), bind.Lazy())
}

// Boot resolves the client and pings the server so a misconfigured connection
// fails fast at startup instead of on the first request.
func (p *MongoProvider) Boot(ctx context.Context, c provider.Container) error {
	var client *mongo.Client
	if err := c.Resolve(&client); err != nil {
		return err
	}

	pingCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	if err := client.Ping(pingCtx, readpref.Primary()); err != nil {
		return err
	}

	p.client = client

	return nil
}

// Terminate disconnects the Mongo client.
func (p *MongoProvider) Terminate(ctx context.Context) error {
	if p.client == nil {
		return nil
	}

	return p.client.Disconnect(ctx)
}
