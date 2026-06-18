// Package http is the presentation layer: it turns HTTP requests into use-case
// calls and renders the results as HTML. It depends on the application layer
// only, never on the repositories directly.
package http

import (
	"errors"
	"net/http"
	"strconv"

	app "github.com/danceable/provider/examples/blog/application/article"
	domain "github.com/danceable/provider/examples/blog/domain/article"
	"github.com/danceable/provider/examples/blog/infrastructure/render"
)

// base holds the dependencies shared by the public and dashboard handlers.
type base struct {
	svc      *app.Service
	renderer *render.Renderer
	perPage  int
}

// view models passed to the templates.
type (
	listView    struct{ Page *app.Page }
	articleView struct{ Article *domain.Article }
	formView    struct {
		Action  string
		Article *domain.Article
		Error   string
	}
	errorView struct {
		Status  int
		Message string
	}
)

// renderError renders the shared error page.
func (b base) renderError(w http.ResponseWriter, status int, message string) {
	b.renderer.Render(w, "error.html", status, errorView{Status: status, Message: message})
}

// isValidation reports whether err is a domain validation error, which the UI
// surfaces back to the user on the form rather than as a 500.
func isValidation(err error) bool {
	return errors.Is(err, domain.ErrEmptyTitle) || errors.Is(err, domain.ErrEmptyBody)
}

// pageParam reads the 1-based ?page query parameter, defaulting to 1.
func pageParam(r *http.Request) int {
	page, _ := strconv.Atoi(r.URL.Query().Get("page"))
	if page < 1 {
		page = 1
	}

	return page
}
