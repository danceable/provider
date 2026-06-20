package memory

import (
	"testing"

	"github.com/danceable/provider/examples/blog/infrastructure/i18n"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestTranslations_AllLanguagesCoverSameKeys guards against a half-translated
// language: every supported language must define exactly the English key set.
func TestTranslations_AllLanguagesCoverSameKeys(t *testing.T) {
	t.Parallel()

	en := translations[i18n.English]
	require.NotEmpty(t, en)

	for _, lang := range i18n.Supported {
		got := translations[lang]
		require.NotEmptyf(t, got, "no translations for language %q", lang)

		for key := range en {
			_, ok := got[key]
			assert.Truef(t, ok, "language %q is missing key %q", lang, key)
		}

		for key := range got {
			_, ok := en[key]
			assert.Truef(t, ok, "language %q has unknown key %q", lang, key)
		}
	}
}

func TestTranslate(t *testing.T) {
	t.Parallel()

	repo := NewTranslationRepository()

	for _, c := range []struct {
		lang i18n.Language
		key  string
		want string
	}{
		{i18n.English, "nav.home", "Home"},
		{i18n.German, "action.save", "Speichern"},
		{i18n.Persian, "nav.home", "خانه"},
		{i18n.Chinese, "dashboard.new", "新建文章"},
	} {
		got, ok := repo.Translate(c.lang, c.key)
		assert.Truef(t, ok, "%s/%s should exist", c.lang, c.key)
		assert.Equal(t, c.want, got)
	}

	_, ok := repo.Translate(i18n.English, "does.not.exist")
	assert.False(t, ok)
}
