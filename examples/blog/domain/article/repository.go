package article

import "context"

// Repository is the persistence port for articles. The domain defines the
// contract; infrastructure adapters (MongoDB, in-memory, ...) implement it.
type Repository interface {
	// Save persists a new article and assigns it an ID.
	Save(ctx context.Context, a *Article) error

	// Update persists changes to an existing article. It returns ErrNotFound
	// when no article with the given ID exists.
	Update(ctx context.Context, a *Article) error

	// Delete removes an article by ID. It returns ErrNotFound when the article
	// does not exist.
	Delete(ctx context.Context, id string) error

	// FindByID returns the article with the given ID, or ErrNotFound.
	FindByID(ctx context.Context, id string) (*Article, error)

	// Paginate returns one page of articles ordered newest-first together with
	// the total number of articles. page is 1-based.
	Paginate(ctx context.Context, page, perPage int) ([]Article, int64, error)
}
