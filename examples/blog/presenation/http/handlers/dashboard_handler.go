package handlers

import (
	"errors"
	"net/http"

	app "github.com/danceable/provider/examples/blog/application/article"
	domain "github.com/danceable/provider/examples/blog/domain/article"
	"github.com/danceable/provider/examples/blog/infrastructure/render"
)

// DashboardHandler serves the admin dashboard: the editable CRUD views.
type DashboardHandler struct {
	base
}

// NewDashboard builds the dashboard handler from the application service and renderer.
func NewDashboard(svc *app.Service, renderer *render.Renderer, perPage int) *DashboardHandler {
	return &DashboardHandler{base{svc: svc, renderer: renderer, perPage: perPage}}
}

// Dashboard renders the admin overview: a paginated, editable list of articles.
func (h *DashboardHandler) Dashboard(w http.ResponseWriter, r *http.Request) {
	page, err := h.svc.List(r.Context(), pageParam(r), h.perPage)
	if err != nil {
		h.renderError(w, r, http.StatusInternalServerError, "could not load articles")
		return
	}

	h.renderer.Render(w, r, "dashboard.html", http.StatusOK, listView{Page: page})
}

// NewForm renders the empty create-article form.
func (h *DashboardHandler) NewForm(w http.ResponseWriter, r *http.Request) {
	h.renderer.Render(w, r, "article_form.html", http.StatusOK, formView{
		Action:  "/dashboard/articles",
		Article: &domain.Article{},
	})
}

// Create handles submission of the create-article form.
func (h *DashboardHandler) Create(w http.ResponseWriter, r *http.Request) {
	title, body := r.FormValue("title"), r.FormValue("body")

	_, err := h.svc.Create(r.Context(), app.CreateInput{Title: title, Body: body})
	switch {
	case err == nil:
		http.Redirect(w, r, "/dashboard", http.StatusSeeOther)
	case isValidation(err):
		h.renderer.Render(w, r, "article_form.html", http.StatusUnprocessableEntity, formView{
			Action:   "/dashboard/articles",
			Article:  &domain.Article{Title: title, Body: body},
			ErrorKey: validationKey(err),
		})
	default:
		h.renderError(w, r, http.StatusInternalServerError, "could not create article")
	}
}

// EditForm renders the edit form pre-filled with an existing article.
func (h *DashboardHandler) EditForm(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")

	a, err := h.svc.Get(r.Context(), id)
	switch {
	case err == nil:
		h.renderer.Render(w, r, "article_form.html", http.StatusOK, formView{
			Action:  "/dashboard/articles/" + id,
			Article: a,
		})
	case errors.Is(err, domain.ErrNotFound):
		h.renderError(w, r, http.StatusNotFound, "article not found")
	default:
		h.renderError(w, r, http.StatusInternalServerError, "could not load article")
	}
}

// Update handles submission of the edit-article form.
func (h *DashboardHandler) Update(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	title, body := r.FormValue("title"), r.FormValue("body")

	_, err := h.svc.Update(r.Context(), app.UpdateInput{ID: id, Title: title, Body: body})
	switch {
	case err == nil:
		http.Redirect(w, r, "/dashboard", http.StatusSeeOther)
	case errors.Is(err, domain.ErrNotFound):
		h.renderError(w, r, http.StatusNotFound, "article not found")
	case isValidation(err):
		h.renderer.Render(w, r, "article_form.html", http.StatusUnprocessableEntity, formView{
			Action:   "/dashboard/articles/" + id,
			Article:  &domain.Article{ID: id, Title: title, Body: body},
			ErrorKey: validationKey(err),
		})
	default:
		h.renderError(w, r, http.StatusInternalServerError, "could not update article")
	}
}

// Delete removes an article and returns to the dashboard.
func (h *DashboardHandler) Delete(w http.ResponseWriter, r *http.Request) {
	err := h.svc.Delete(r.Context(), r.PathValue("id"))
	if err != nil && !errors.Is(err, domain.ErrNotFound) {
		h.renderError(w, r, http.StatusInternalServerError, "could not delete article")
		return
	}

	http.Redirect(w, r, "/dashboard", http.StatusSeeOther)
}
