package handlers_test

import (
	"io"
	"net/http"
	"testing"

	app "github.com/danceable/provider/examples/blog/application/article"
	"github.com/danceable/provider/examples/blog/infrastructure/i18n"
	"github.com/danceable/provider/examples/blog/infrastructure/render"
	"github.com/danceable/provider/examples/blog/infrastructure/repositories/memory"
	"github.com/stretchr/testify/require"
)

// newService returns an article service backed by an in-memory repository, so
// the handler tests need no MongoDB instance.
func newService() *app.Service {
	return app.NewService(memory.NewArticleRepository())
}

// englishTranslator builds a default English translator over the in-memory
// translations, so handlers tested without the i18n middleware still render
// real text rather than raw keys.
func englishTranslator() *i18n.Translator {
	return i18n.NewTranslator(memory.NewTranslationRepository(), i18n.English)
}

// newRenderer compiles the templates the handlers render against, defaulting to
// English when a request carries no translator.
func newRenderer(t *testing.T) *render.Renderer {
	t.Helper()

	renderer, err := render.New(englishTranslator())
	require.NoError(t, err)

	return renderer
}

// noRedirectClient returns a client that surfaces redirects instead of following them.
func noRedirectClient() *http.Client {
	return &http.Client{
		CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse },
	}
}

// readBody reads and returns the response body as a string.
func readBody(t *testing.T, res *http.Response) string {
	t.Helper()

	body, err := io.ReadAll(res.Body)
	require.NoError(t, err)

	return string(body)
}
