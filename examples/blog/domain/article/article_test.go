package article_test

import (
	"testing"

	"github.com/danceable/provider/examples/blog/domain/article"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNew(t *testing.T) {
	t.Parallel()

	t.Run("valid", func(t *testing.T) {
		t.Parallel()

		a, err := article.New("  Hello  ", "  World  ")
		require.NoError(t, err)
		assert.Empty(t, a.ID, "identity is assigned by the repository")
		assert.Equal(t, "Hello", a.Title, "title is trimmed")
		assert.Equal(t, "World", a.Body, "body is trimmed")
		assert.False(t, a.CreatedAt.IsZero())
	})

	t.Run("empty title", func(t *testing.T) {
		t.Parallel()

		_, err := article.New("   ", "body")
		assert.ErrorIs(t, err, article.ErrEmptyTitle)
	})

	t.Run("empty body", func(t *testing.T) {
		t.Parallel()

		_, err := article.New("title", "   ")
		assert.ErrorIs(t, err, article.ErrEmptyBody)
	})
}

func TestSetters(t *testing.T) {
	t.Parallel()

	a, err := article.New("title", "body")
	require.NoError(t, err)

	assert.ErrorIs(t, a.SetTitle(""), article.ErrEmptyTitle)
	assert.Equal(t, "title", a.Title, "title is unchanged after a rejected update")

	assert.ErrorIs(t, a.SetBody(""), article.ErrEmptyBody)
	assert.Equal(t, "body", a.Body, "body is unchanged after a rejected update")

	require.NoError(t, a.SetTitle("new title"))
	require.NoError(t, a.SetBody("new body"))
	assert.Equal(t, "new title", a.Title)
	assert.Equal(t, "new body", a.Body)
}
