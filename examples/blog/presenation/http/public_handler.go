package http

import (
	"errors"
	"net/http"

	app "github.com/danceable/provider/examples/blog/application/article"
	domain "github.com/danceable/provider/examples/blog/domain/article"
	"github.com/danceable/provider/examples/blog/infrastructure/render"
)

// PublicHandler serves the public, read-only pages of the blog.
type PublicHandler struct {
	base
}

// NewPublic builds the public handler from the application service and renderer.
func NewPublic(svc *app.Service, renderer *render.Renderer, perPage int) *PublicHandler {
	return &PublicHandler{base{svc: svc, renderer: renderer, perPage: perPage}}
}

// Home renders the public landing page: a paginated list of articles.
func (h *PublicHandler) Home(w http.ResponseWriter, r *http.Request) {
	page, err := h.svc.List(r.Context(), pageParam(r), h.perPage)
	if err != nil {
		h.renderError(w, http.StatusInternalServerError, "could not load articles")
		return
	}

	h.renderer.Render(w, "home.html", http.StatusOK, listView{Page: page})
}

// Show renders a single article's detail page.
func (h *PublicHandler) Show(w http.ResponseWriter, r *http.Request) {
	a, err := h.svc.Get(r.Context(), r.PathValue("id"))
	switch {
	case err == nil:
		h.renderer.Render(w, "article.html", http.StatusOK, articleView{Article: a})
	case errors.Is(err, domain.ErrNotFound):
		h.renderError(w, http.StatusNotFound, "article not found")
	default:
		h.renderError(w, http.StatusInternalServerError, "could not load article")
	}
}
