package article_test

import (
	"context"
	"testing"

	app "github.com/danceable/provider/examples/blog/application/article"
	domain "github.com/danceable/provider/examples/blog/domain/article"
	"github.com/danceable/provider/examples/blog/infrastructure/memory"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newService() *app.Service {
	return app.NewService(memory.NewArticleRepository())
}

func TestService_Create(t *testing.T) {
	t.Parallel()

	svc := newService()

	a, err := svc.Create(context.Background(), app.CreateInput{Title: "Title", Body: "Body"})
	require.NoError(t, err)
	assert.NotEmpty(t, a.ID, "repository assigns an ID")
	assert.Equal(t, "Title", a.Title)

	_, err = svc.Create(context.Background(), app.CreateInput{Title: "", Body: "Body"})
	assert.ErrorIs(t, err, domain.ErrEmptyTitle)
}

func TestService_Get(t *testing.T) {
	t.Parallel()

	svc := newService()

	created, err := svc.Create(context.Background(), app.CreateInput{Title: "T", Body: "B"})
	require.NoError(t, err)

	got, err := svc.Get(context.Background(), created.ID)
	require.NoError(t, err)
	assert.Equal(t, created.ID, got.ID)

	_, err = svc.Get(context.Background(), "missing")
	assert.ErrorIs(t, err, domain.ErrNotFound)
}

func TestService_Update(t *testing.T) {
	t.Parallel()

	svc := newService()

	created, err := svc.Create(context.Background(), app.CreateInput{Title: "Old", Body: "Old"})
	require.NoError(t, err)

	updated, err := svc.Update(context.Background(), app.UpdateInput{ID: created.ID, Title: "New", Body: "New body"})
	require.NoError(t, err)
	assert.Equal(t, "New", updated.Title)
	assert.Equal(t, "New body", updated.Body)

	// Invariants are re-checked on update.
	_, err = svc.Update(context.Background(), app.UpdateInput{ID: created.ID, Title: "", Body: "x"})
	assert.ErrorIs(t, err, domain.ErrEmptyTitle)

	// Unknown article.
	_, err = svc.Update(context.Background(), app.UpdateInput{ID: "missing", Title: "a", Body: "b"})
	assert.ErrorIs(t, err, domain.ErrNotFound)
}

func TestService_Delete(t *testing.T) {
	t.Parallel()

	svc := newService()

	created, err := svc.Create(context.Background(), app.CreateInput{Title: "T", Body: "B"})
	require.NoError(t, err)

	require.NoError(t, svc.Delete(context.Background(), created.ID))
	assert.ErrorIs(t, svc.Delete(context.Background(), created.ID), domain.ErrNotFound)
}

func TestService_List_Pagination(t *testing.T) {
	t.Parallel()

	svc := newService()

	const total = 12
	for i := 0; i < total; i++ {
		_, err := svc.Create(context.Background(), app.CreateInput{Title: "T", Body: "B"})
		require.NoError(t, err)
	}

	first, err := svc.List(context.Background(), 1, 5)
	require.NoError(t, err)
	assert.Len(t, first.Articles, 5)
	assert.EqualValues(t, total, first.Total)
	assert.Equal(t, 3, first.TotalPages) // ceil(12/5)
	assert.False(t, first.HasPrev)
	assert.True(t, first.HasNext)

	last, err := svc.List(context.Background(), 3, 5)
	require.NoError(t, err)
	assert.Len(t, last.Articles, 2) // remainder
	assert.True(t, last.HasPrev)
	assert.False(t, last.HasNext)

	// Out-of-range and zero values are clamped instead of erroring.
	beyond, err := svc.List(context.Background(), 99, 0)
	require.NoError(t, err)
	assert.Empty(t, beyond.Articles)
	assert.EqualValues(t, total, beyond.Total)
}
