package http_test

import (
	"io"
	"net/http"
	"testing"

	app "github.com/danceable/provider/examples/blog/application/article"
	"github.com/danceable/provider/examples/blog/infrastructure/memory"
	"github.com/danceable/provider/examples/blog/infrastructure/render"
	"github.com/stretchr/testify/require"
)

// newService returns an article service backed by an in-memory repository, so
// the handler tests need no MongoDB instance.
func newService() *app.Service {
	return app.NewService(memory.NewArticleRepository())
}

// newRenderer compiles the templates the handlers render against.
func newRenderer(t *testing.T) *render.Renderer {
	t.Helper()

	renderer, err := render.New()
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
