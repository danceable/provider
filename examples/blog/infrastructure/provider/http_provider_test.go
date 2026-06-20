package provider_test

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/danceable/provider"
	app "github.com/danceable/provider/examples/blog/application/article"
	"github.com/danceable/provider/examples/blog/infrastructure/i18n"
	blogprovider "github.com/danceable/provider/examples/blog/infrastructure/provider"
	"github.com/danceable/provider/examples/blog/infrastructure/repositories/memory"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// noScope is a middlewares.Scoper that always fails to open a scope. The i18n
// middleware then falls back to the renderer's default translator, which lets
// the test exercise NewServer without standing up the scoped provider graph.
type noScope struct{}

func (noScope) Scope(context.Context, ...provider.ScopeOption) (*provider.Scope, error) {
	return nil, errors.New("no scope in test")
}

func TestHTTPProvider_Lifecycle(t *testing.T) {
	t.Parallel()

	p := blogprovider.NewHTTPProvider()
	assert.Equal(t, 30, p.Order(), "HTTP boots last")

	// Terminate before Boot has no server to shut down.
	require.NoError(t, p.Terminate(context.Background()))
}

// TestNewServer asserts the handler assembly: NewServer wires the public and
// dashboard routes over the application service and answers a request.
func TestNewServer(t *testing.T) {
	t.Parallel()

	svc := app.NewService(memory.NewArticleRepository())
	translator := i18n.NewTranslator(memory.NewTranslationRepository(), i18n.Default)

	handler, err := blogprovider.NewServer(svc, 5, noScope{}, translator)
	require.NoError(t, err)
	require.NotNil(t, handler)

	for _, path := range []string{"/", "/dashboard", "/dashboard/articles/new"} {
		req := httptest.NewRequest(http.MethodGet, path, nil)
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)
		assert.Equalf(t, http.StatusOK, rec.Code, "GET %s", path)
	}
}
