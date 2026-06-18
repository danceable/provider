package provider

import (
	"context"

	"github.com/danceable/container/bind"
	"github.com/danceable/provider"
	app "github.com/danceable/provider/examples/blog/application/article"
	domain "github.com/danceable/provider/examples/blog/domain/article"
	"github.com/danceable/provider/examples/blog/infrastructure/repositories/mongodb"
	"go.mongodb.org/mongo-driver/v2/mongo"
)

// ArticleProvider wires the article domain: it binds the MongoDB repository to
// the domain.Repository port and the application service on top of it. This is
// where the dependency-inversion seam is closed — callers depend on the port,
// the container supplies the MongoDB adapter.
type ArticleProvider struct{}

var _ provider.Provider = (*ArticleProvider)(nil)

// NewArticleProvider creates an ArticleProvider.
func NewArticleProvider() *ArticleProvider { return &ArticleProvider{} }

// Order places domain wiring after the database is available.
func (p *ArticleProvider) Order() int { return 20 }

// Register binds the repository port and the use-case service.
func (p *ArticleProvider) Register(_ context.Context, c provider.Container) error {
	if err := c.Bind(func(db *mongo.Database) domain.Repository {
		return mongodb.NewArticleRepository(db)
	}, bind.Singleton(), bind.Lazy()); err != nil {
		return err
	}

	return c.Bind(func(repo domain.Repository) *app.Service {
		return app.NewService(repo)
	}, bind.Singleton(), bind.Lazy())
}

// Boot has nothing to do.
func (p *ArticleProvider) Boot(_ context.Context, _ provider.Container) error { return nil }

// Terminate has nothing to release.
func (p *ArticleProvider) Terminate(_ context.Context) error { return nil }
