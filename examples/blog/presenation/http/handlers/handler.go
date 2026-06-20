// Package handlers is the presentation layer: it turns HTTP requests into
// use-case calls and renders the results as HTML. It depends on the application
// layer only, never on the repositories directly.
package handlers

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
		// ErrorKey is a translation key for the validation message, so the
		// template renders it in the request's language via {{ t }}.
		ErrorKey string
	}
	errorView struct {
		Status  int
		Message string
	}
)

// renderError renders the shared error page in the request's language.
func (b base) renderError(w http.ResponseWriter, r *http.Request, status int, message string) {
	b.renderer.Render(w, r, "error.html", status, errorView{Status: status, Message: message})
}

// isValidation reports whether err is a domain validation error, which the UI
// surfaces back to the user on the form rather than as a 500.
func isValidation(err error) bool {
	return errors.Is(err, domain.ErrEmptyTitle) || errors.Is(err, domain.ErrEmptyBody)
}

// validationKey maps a domain validation error to its translation key, so the
// form template can render the message in the request's language. It is only
// called for errors isValidation has already accepted.
func validationKey(err error) string {
	switch {
	case errors.Is(err, domain.ErrEmptyTitle):
		return "error.empty_title"
	case errors.Is(err, domain.ErrEmptyBody):
		return "error.empty_body"
	default:
		return "error.invalid"
	}
}

// pageParam reads the 1-based ?page query parameter, defaulting to 1.
func pageParam(r *http.Request) int {
	page, _ := strconv.Atoi(r.URL.Query().Get("page"))
	if page < 1 {
		page = 1
	}

	return page
}
