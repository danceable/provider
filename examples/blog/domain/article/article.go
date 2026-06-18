// Package article is the domain layer for blog articles. It owns the Article
// entity, its invariants and the Repository port. It has no knowledge of how
// articles are stored or delivered, so it never imports MongoDB, HTTP or the
// container.
package article

import (
	"errors"
	"strings"
	"time"
)

// Domain errors. They are part of the ubiquitous language and are returned to
// the outer layers so they can be mapped to transport-specific responses
// (e.g. a 404 for ErrNotFound).
var (
	// ErrNotFound is returned when an article cannot be located.
	ErrNotFound = errors.New("article: not found")

	// ErrEmptyTitle is returned when an article is given a blank title.
	ErrEmptyTitle = errors.New("article: title must not be empty")

	// ErrEmptyBody is returned when an article is given a blank body.
	ErrEmptyBody = errors.New("article: body must not be empty")
)

// Article is the aggregate root of the blog. Its fields mirror the requested
// shape: id, title, body and created_at.
type Article struct {
	ID        string
	Title     string
	Body      string
	CreatedAt time.Time
}

// New creates a valid Article. Identity is assigned by the repository when the
// article is persisted, so a freshly created article has an empty ID.
func New(title, body string) (*Article, error) {
	a := &Article{CreatedAt: time.Now().UTC()}

	if err := a.SetTitle(title); err != nil {
		return nil, err
	}

	if err := a.SetBody(body); err != nil {
		return nil, err
	}

	return a, nil
}

// SetTitle validates and updates the title, enforcing the non-empty invariant.
func (a *Article) SetTitle(title string) error {
	title = strings.TrimSpace(title)
	if title == "" {
		return ErrEmptyTitle
	}

	a.Title = title

	return nil
}

// SetBody validates and updates the body, enforcing the non-empty invariant.
func (a *Article) SetBody(body string) error {
	body = strings.TrimSpace(body)
	if body == "" {
		return ErrEmptyBody
	}

	a.Body = body

	return nil
}
