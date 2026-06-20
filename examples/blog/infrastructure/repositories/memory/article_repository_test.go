package memory

import (
	"context"
	"testing"
	"time"

	domain "github.com/danceable/provider/examples/blog/domain/article"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestArticleRepository_Save(t *testing.T) {
	t.Parallel()

	t.Run("assigns an ID when missing", func(t *testing.T) {
		t.Parallel()

		repo := NewArticleRepository()
		a := &domain.Article{Title: "Title", Body: "Body"}

		require.NoError(t, repo.Save(context.Background(), a))
		assert.NotEmpty(t, a.ID, "Save assigns a random ID")

		got, err := repo.FindByID(context.Background(), a.ID)
		require.NoError(t, err)
		assert.Equal(t, "Title", got.Title)
		assert.Equal(t, "Body", got.Body)
	})

	t.Run("keeps a pre-assigned ID", func(t *testing.T) {
		t.Parallel()

		repo := NewArticleRepository()
		a := &domain.Article{ID: "fixed-id", Title: "Title", Body: "Body"}

		require.NoError(t, repo.Save(context.Background(), a))
		assert.Equal(t, "fixed-id", a.ID)

		_, err := repo.FindByID(context.Background(), "fixed-id")
		require.NoError(t, err)
	})
}

func TestArticleRepository_Update(t *testing.T) {
	t.Parallel()

	t.Run("replaces an existing article", func(t *testing.T) {
		t.Parallel()

		repo := NewArticleRepository()
		a := &domain.Article{Title: "Title", Body: "Body"}
		require.NoError(t, repo.Save(context.Background(), a))

		a.Title = "Updated"
		require.NoError(t, repo.Update(context.Background(), a))

		got, err := repo.FindByID(context.Background(), a.ID)
		require.NoError(t, err)
		assert.Equal(t, "Updated", got.Title)
	})

	t.Run("returns ErrNotFound for an unknown article", func(t *testing.T) {
		t.Parallel()

		repo := NewArticleRepository()
		err := repo.Update(context.Background(), &domain.Article{ID: "missing"})
		assert.ErrorIs(t, err, domain.ErrNotFound)
	})
}

func TestArticleRepository_Delete(t *testing.T) {
	t.Parallel()

	t.Run("removes an existing article", func(t *testing.T) {
		t.Parallel()

		repo := NewArticleRepository()
		a := &domain.Article{Title: "Title", Body: "Body"}
		require.NoError(t, repo.Save(context.Background(), a))

		require.NoError(t, repo.Delete(context.Background(), a.ID))

		_, err := repo.FindByID(context.Background(), a.ID)
		assert.ErrorIs(t, err, domain.ErrNotFound)
	})

	t.Run("returns ErrNotFound for an unknown article", func(t *testing.T) {
		t.Parallel()

		repo := NewArticleRepository()
		err := repo.Delete(context.Background(), "missing")
		assert.ErrorIs(t, err, domain.ErrNotFound)
	})
}

func TestArticleRepository_FindByID(t *testing.T) {
	t.Parallel()

	t.Run("returns a copy, not the stored value", func(t *testing.T) {
		t.Parallel()

		repo := NewArticleRepository()
		a := &domain.Article{Title: "Title", Body: "Body"}
		require.NoError(t, repo.Save(context.Background(), a))

		got, err := repo.FindByID(context.Background(), a.ID)
		require.NoError(t, err)

		// Mutating the returned article must not affect stored state.
		got.Title = "Mutated"

		again, err := repo.FindByID(context.Background(), a.ID)
		require.NoError(t, err)
		assert.Equal(t, "Title", again.Title)
	})

	t.Run("returns ErrNotFound for an unknown article", func(t *testing.T) {
		t.Parallel()

		repo := NewArticleRepository()
		_, err := repo.FindByID(context.Background(), "missing")
		assert.ErrorIs(t, err, domain.ErrNotFound)
	})
}

func TestArticleRepository_Paginate(t *testing.T) {
	t.Parallel()

	// seed inserts n articles with strictly increasing timestamps so the
	// newest-first ordering is deterministic.
	seed := func(repo *ArticleRepository, n int) []domain.Article {
		base := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
		out := make([]domain.Article, 0, n)
		for i := 0; i < n; i++ {
			a := &domain.Article{
				Title:     "Title",
				Body:      "Body",
				CreatedAt: base.Add(time.Duration(i) * time.Hour),
			}
			require.NoError(t, repo.Save(context.Background(), a))
			out = append(out, *a)
		}
		return out
	}

	t.Run("orders newest first and reports the total", func(t *testing.T) {
		t.Parallel()

		repo := NewArticleRepository()
		seeded := seed(repo, 3)

		got, total, err := repo.Paginate(context.Background(), 1, 10)
		require.NoError(t, err)
		assert.Equal(t, int64(3), total)
		require.Len(t, got, 3)

		// seeded[2] is newest.
		assert.Equal(t, seeded[2].ID, got[0].ID)
		assert.Equal(t, seeded[1].ID, got[1].ID)
		assert.Equal(t, seeded[0].ID, got[2].ID)
	})

	t.Run("returns the requested page", func(t *testing.T) {
		t.Parallel()

		repo := NewArticleRepository()
		seed(repo, 5)

		got, total, err := repo.Paginate(context.Background(), 2, 2)
		require.NoError(t, err)
		assert.Equal(t, int64(5), total)
		assert.Len(t, got, 2)
	})

	t.Run("returns an empty page past the end", func(t *testing.T) {
		t.Parallel()

		repo := NewArticleRepository()
		seed(repo, 2)

		got, total, err := repo.Paginate(context.Background(), 5, 10)
		require.NoError(t, err)
		assert.Equal(t, int64(2), total)
		assert.Empty(t, got)
	})

	t.Run("caps the final partial page", func(t *testing.T) {
		t.Parallel()

		repo := NewArticleRepository()
		seed(repo, 3)

		got, total, err := repo.Paginate(context.Background(), 2, 2)
		require.NoError(t, err)
		assert.Equal(t, int64(3), total)
		assert.Len(t, got, 1, "only one article remains on the last page")
	})

	t.Run("empty repository", func(t *testing.T) {
		t.Parallel()

		repo := NewArticleRepository()
		got, total, err := repo.Paginate(context.Background(), 1, 10)
		require.NoError(t, err)
		assert.Zero(t, total)
		assert.Empty(t, got)
	})
}
