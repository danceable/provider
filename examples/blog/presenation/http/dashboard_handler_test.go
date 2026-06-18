package http_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	app "github.com/danceable/provider/examples/blog/application/article"
	bloghttp "github.com/danceable/provider/examples/blog/presenation/http"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// newDashboardServer wires the dashboard handler to an in-memory repository and
// returns the running test server plus the service for arranging fixtures.
func newDashboardServer(t *testing.T) (*httptest.Server, *app.Service) {
	t.Helper()

	svc := newService()
	h := bloghttp.NewDashboard(svc, newRenderer(t), 5)

	mux := http.NewServeMux()
	mux.HandleFunc("GET /dashboard", h.Dashboard)
	mux.HandleFunc("GET /dashboard/articles/new", h.NewForm)
	mux.HandleFunc("POST /dashboard/articles", h.Create)
	mux.HandleFunc("GET /dashboard/articles/{id}/edit", h.EditForm)
	mux.HandleFunc("POST /dashboard/articles/{id}", h.Update)
	mux.HandleFunc("POST /dashboard/articles/{id}/delete", h.Delete)

	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)

	return srv, svc
}

func TestDashboard(t *testing.T) {
	t.Parallel()

	srv, svc := newDashboardServer(t)
	_, err := svc.Create(context.Background(), app.CreateInput{Title: "Listed", Body: "Body"})
	require.NoError(t, err)

	res, err := http.Get(srv.URL + "/dashboard")
	require.NoError(t, err)
	defer res.Body.Close()

	assert.Equal(t, http.StatusOK, res.StatusCode)
	body := readBody(t, res)
	assert.Contains(t, body, "Listed")
	assert.Contains(t, body, "New article")
}

func TestCreate(t *testing.T) {
	t.Parallel()

	srv, svc := newDashboardServer(t)
	client := noRedirectClient()

	t.Run("valid redirects to dashboard", func(t *testing.T) {
		res, err := client.PostForm(srv.URL+"/dashboard/articles", url.Values{
			"title": {"Brand new"},
			"body":  {"Some content"},
		})
		require.NoError(t, err)
		defer res.Body.Close()

		assert.Equal(t, http.StatusSeeOther, res.StatusCode)
		assert.Equal(t, "/dashboard", res.Header.Get("Location"))

		page, err := svc.List(context.Background(), 1, 10)
		require.NoError(t, err)
		assert.EqualValues(t, 1, page.Total)
	})

	t.Run("invalid re-renders the form", func(t *testing.T) {
		res, err := client.PostForm(srv.URL+"/dashboard/articles", url.Values{
			"title": {""},
			"body":  {"Some content"},
		})
		require.NoError(t, err)
		defer res.Body.Close()

		assert.Equal(t, http.StatusUnprocessableEntity, res.StatusCode)
		assert.Contains(t, readBody(t, res), "title must not be empty")
	})
}

func TestUpdate(t *testing.T) {
	t.Parallel()

	srv, svc := newDashboardServer(t)
	created, err := svc.Create(context.Background(), app.CreateInput{Title: "Before", Body: "Before body"})
	require.NoError(t, err)

	client := noRedirectClient()
	res, err := client.PostForm(srv.URL+"/dashboard/articles/"+created.ID, url.Values{
		"title": {"After"},
		"body":  {"After body"},
	})
	require.NoError(t, err)
	defer res.Body.Close()

	assert.Equal(t, http.StatusSeeOther, res.StatusCode)

	got, err := svc.Get(context.Background(), created.ID)
	require.NoError(t, err)
	assert.Equal(t, "After", got.Title)
}

func TestDelete(t *testing.T) {
	t.Parallel()

	srv, svc := newDashboardServer(t)
	created, err := svc.Create(context.Background(), app.CreateInput{Title: "Doomed", Body: "Body"})
	require.NoError(t, err)

	client := noRedirectClient()
	res, err := client.PostForm(srv.URL+"/dashboard/articles/"+created.ID+"/delete", nil)
	require.NoError(t, err)
	defer res.Body.Close()

	assert.Equal(t, http.StatusSeeOther, res.StatusCode)

	page, err := svc.List(context.Background(), 1, 10)
	require.NoError(t, err)
	assert.EqualValues(t, 0, page.Total)
}
