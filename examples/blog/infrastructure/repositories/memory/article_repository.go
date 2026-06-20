// Package memory provides an in-memory implementation of the article
// repository port. It is handy for tests and local experimentation without a
// running MongoDB instance.
package memory

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"sort"
	"sync"

	domain "github.com/danceable/provider/examples/blog/domain/article"
)

// ArticleRepository is a goroutine-safe, in-memory domain.Repository.
type ArticleRepository struct {
	mu    sync.RWMutex
	items map[string]domain.Article
}

// compile-time assertion that the adapter satisfies the domain port.
var _ domain.Repository = (*ArticleRepository)(nil)

// NewArticleRepository returns an empty in-memory repository.
func NewArticleRepository() *ArticleRepository {
	return &ArticleRepository{items: make(map[string]domain.Article)}
}

// Save stores a new article, assigning it a random ID.
func (r *ArticleRepository) Save(_ context.Context, a *domain.Article) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if a.ID == "" {
		a.ID = newID()
	}
	r.items[a.ID] = *a

	return nil
}

// Update replaces an existing article.
func (r *ArticleRepository) Update(_ context.Context, a *domain.Article) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, ok := r.items[a.ID]; !ok {
		return domain.ErrNotFound
	}
	r.items[a.ID] = *a

	return nil
}

// Delete removes an article by ID.
func (r *ArticleRepository) Delete(_ context.Context, id string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, ok := r.items[id]; !ok {
		return domain.ErrNotFound
	}
	delete(r.items, id)

	return nil
}

// FindByID returns a copy of the stored article.
func (r *ArticleRepository) FindByID(_ context.Context, id string) (*domain.Article, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	a, ok := r.items[id]
	if !ok {
		return nil, domain.ErrNotFound
	}

	return &a, nil
}

// Paginate returns one newest-first page of articles and the total count.
func (r *ArticleRepository) Paginate(_ context.Context, page, perPage int) ([]domain.Article, int64, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	all := make([]domain.Article, 0, len(r.items))
	for _, a := range r.items {
		all = append(all, a)
	}

	// Newest first; fall back to ID so the order is deterministic when two
	// articles share a timestamp (common in fast tests).
	sort.Slice(all, func(i, j int) bool {
		if all[i].CreatedAt.Equal(all[j].CreatedAt) {
			return all[i].ID > all[j].ID
		}
		return all[i].CreatedAt.After(all[j].CreatedAt)
	})

	total := int64(len(all))

	start := (page - 1) * perPage
	if start >= len(all) {
		return []domain.Article{}, total, nil
	}

	end := start + perPage
	if end > len(all) {
		end = len(all)
	}

	return append([]domain.Article{}, all[start:end]...), total, nil
}

// newID returns a random 96-bit hex identifier.
func newID() string {
	b := make([]byte, 12)
	_, _ = rand.Read(b)

	return hex.EncodeToString(b)
}
