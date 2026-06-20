package handlers_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	app "github.com/danceable/provider/examples/blog/application/article"
	"github.com/danceable/provider/examples/blog/presenation/http/handlers"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// newPublicServer wires the public handler to an in-memory repository and
// returns the running test server plus the service for arranging fixtures.
func newPublicServer(t *testing.T) (*httptest.Server, *app.Service) {
	t.Helper()

	svc := newService()
	h := handlers.NewPublic(svc, newRenderer(t), 5)

	mux := http.NewServeMux()
	mux.HandleFunc("GET /{$}", h.Home)
	mux.HandleFunc("GET /articles/{id}", h.Show)

	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)

	return srv, svc
}

func TestHome(t *testing.T) {
	t.Parallel()

	srv, svc := newPublicServer(t)
	_, err := svc.Create(context.Background(), app.CreateInput{Title: "Hello World", Body: "Body"})
	require.NoError(t, err)

	res, err := http.Get(srv.URL + "/")
	require.NoError(t, err)
	defer res.Body.Close()

	assert.Equal(t, http.StatusOK, res.StatusCode)
	assert.Contains(t, readBody(t, res), "Hello World")
}

func TestShow(t *testing.T) {
	t.Parallel()

	srv, svc := newPublicServer(t)
	created, err := svc.Create(context.Background(), app.CreateInput{Title: "Detail", Body: "Full body here"})
	require.NoError(t, err)

	t.Run("found", func(t *testing.T) {
		res, err := http.Get(srv.URL + "/articles/" + created.ID)
		require.NoError(t, err)
		defer res.Body.Close()

		assert.Equal(t, http.StatusOK, res.StatusCode)
		assert.Contains(t, readBody(t, res), "Full body here")
	})

	t.Run("missing", func(t *testing.T) {
		res, err := http.Get(srv.URL + "/articles/does-not-exist")
		require.NoError(t, err)
		defer res.Body.Close()

		assert.Equal(t, http.StatusNotFound, res.StatusCode)
	})
}
