// Package article is the application layer for articles. It exposes the use
// cases (create, read, update, delete, list) that orchestrate the domain and
// the repository port. It depends on the domain but never on infrastructure
// or transport details.
package article

import (
	"context"

	domain "github.com/danceable/provider/examples/blog/domain/article"
)

// Service implements the article use cases on top of a domain.Repository.
type Service struct {
	repo domain.Repository
}

// NewService wires the use cases to a repository implementation.
func NewService(repo domain.Repository) *Service {
	return &Service{repo: repo}
}

// CreateInput carries the data needed to create an article.
type CreateInput struct {
	Title string
	Body  string
}

// Create validates and stores a new article.
func (s *Service) Create(ctx context.Context, in CreateInput) (*domain.Article, error) {
	a, err := domain.New(in.Title, in.Body)
	if err != nil {
		return nil, err
	}

	if err := s.repo.Save(ctx, a); err != nil {
		return nil, err
	}

	return a, nil
}

// UpdateInput carries the data needed to update an existing article.
type UpdateInput struct {
	ID    string
	Title string
	Body  string
}

// Update applies changes to an existing article after re-validating its
// invariants. It returns domain.ErrNotFound when the article does not exist.
func (s *Service) Update(ctx context.Context, in UpdateInput) (*domain.Article, error) {
	a, err := s.repo.FindByID(ctx, in.ID)
	if err != nil {
		return nil, err
	}

	if err := a.SetTitle(in.Title); err != nil {
		return nil, err
	}

	if err := a.SetBody(in.Body); err != nil {
		return nil, err
	}

	if err := s.repo.Update(ctx, a); err != nil {
		return nil, err
	}

	return a, nil
}

// Delete removes an article by ID.
func (s *Service) Delete(ctx context.Context, id string) error {
	return s.repo.Delete(ctx, id)
}

// Get returns a single article by ID.
func (s *Service) Get(ctx context.Context, id string) (*domain.Article, error) {
	return s.repo.FindByID(ctx, id)
}

// Page is a paginated slice of articles enriched with the navigation metadata
// the presentation layer needs to render pagination controls.
type Page struct {
	Articles   []domain.Article
	Total      int64
	Page       int
	PerPage    int
	TotalPages int
	HasPrev    bool
	HasNext    bool
}

// List returns the requested page of articles. page and perPage are clamped to
// sane lower bounds so callers can pass raw, user-supplied values.
func (s *Service) List(ctx context.Context, page, perPage int) (*Page, error) {
	if page < 1 {
		page = 1
	}

	if perPage < 1 {
		perPage = 10
	}

	articles, total, err := s.repo.Paginate(ctx, page, perPage)
	if err != nil {
		return nil, err
	}

	totalPages := int((total + int64(perPage) - 1) / int64(perPage))

	return &Page{
		Articles:   articles,
		Total:      total,
		Page:       page,
		PerPage:    perPage,
		TotalPages: totalPages,
		HasPrev:    page > 1,
		HasNext:    page < totalPages,
	}, nil
}
