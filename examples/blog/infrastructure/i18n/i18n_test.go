package i18n_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/danceable/provider/examples/blog/infrastructure/i18n"
	"github.com/stretchr/testify/assert"
)

// fakeRepo is a tiny in-test Repository for exercising the Translator fallbacks.
type fakeRepo map[i18n.Language]map[string]string

func (f fakeRepo) Translate(lang i18n.Language, key string) (string, bool) {
	v, ok := f[lang][key]
	return v, ok
}

func TestTranslator_T_FallsBackThroughDefaultToKey(t *testing.T) {
	t.Parallel()

	repo := fakeRepo{
		i18n.English: {"greet": "Hello"},
		i18n.German:  {"greet": "Hallo"},
	}

	// Present in the requested language.
	assert.Equal(t, "Hallo", i18n.NewTranslator(repo, i18n.German).T("greet"))

	// Missing in the requested language → falls back to the default language.
	assert.Equal(t, "Hello", i18n.NewTranslator(repo, i18n.Persian).T("greet"))

	// Missing everywhere → returns the key itself.
	assert.Equal(t, "unknown.key", i18n.NewTranslator(repo, i18n.German).T("unknown.key"))
}

func TestTranslator_Dir(t *testing.T) {
	t.Parallel()

	repo := fakeRepo{}
	assert.Equal(t, "rtl", i18n.NewTranslator(repo, i18n.Persian).Dir())
	assert.Equal(t, "ltr", i18n.NewTranslator(repo, i18n.English).Dir())
	assert.Equal(t, "ltr", i18n.NewTranslator(repo, i18n.Chinese).Dir())
}

func TestParse(t *testing.T) {
	t.Parallel()

	for code, want := range map[string]i18n.Language{
		"en": i18n.English, "DE": i18n.German, " fa ": i18n.Persian, "zh": i18n.Chinese,
	} {
		got, ok := i18n.Parse(code)
		assert.Truef(t, ok, "expected %q to parse", code)
		assert.Equal(t, want, got)
	}

	_, ok := i18n.Parse("xx")
	assert.False(t, ok)
}

func TestDetect(t *testing.T) {
	t.Parallel()

	t.Run("query parameter wins", func(t *testing.T) {
		r := httptest.NewRequest(http.MethodGet, "/?lang=fa", nil)
		assert.Equal(t, i18n.Persian, i18n.Detect(r))
	})

	t.Run("cookie when no query", func(t *testing.T) {
		r := httptest.NewRequest(http.MethodGet, "/", nil)
		r.AddCookie(&http.Cookie{Name: "lang", Value: "de"})
		assert.Equal(t, i18n.German, i18n.Detect(r))
	})

	t.Run("accept-language header", func(t *testing.T) {
		r := httptest.NewRequest(http.MethodGet, "/", nil)
		r.Header.Set("Accept-Language", "zh-CN,zh;q=0.9,en;q=0.8")
		assert.Equal(t, i18n.Chinese, i18n.Detect(r))
	})

	t.Run("default when nothing matches", func(t *testing.T) {
		r := httptest.NewRequest(http.MethodGet, "/", nil)
		r.Header.Set("Accept-Language", "xx,yy")
		assert.Equal(t, i18n.Default, i18n.Detect(r))
	})
}

func TestContextRoundTrip(t *testing.T) {
	t.Parallel()

	tr := i18n.NewTranslator(fakeRepo{}, i18n.German)
	ctx := i18n.WithTranslator(context.Background(), tr)

	assert.Same(t, tr, i18n.FromContext(ctx))
	assert.Nil(t, i18n.FromContext(context.Background()))
}
