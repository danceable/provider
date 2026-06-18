// Package render is the HTML rendering infrastructure: it compiles the embedded
// templates once and renders page+layout pairs to an http.ResponseWriter. It is
// a technical adapter, so the presentation layer depends on it rather than on
// html/template directly.
package render

import (
	"bytes"
	"html/template"
	"net/http"

	"github.com/danceable/provider/examples/blog/resources"
)

// pages lists the page templates. Each one is compiled together with the
// shared layout into its own template set so their "content"/"title" blocks
// don't collide.
var pages = []string{
	"home.html",
	"article.html",
	"dashboard.html",
	"article_form.html",
	"error.html",
}

// funcs are the helpers available inside every template.
var funcs = template.FuncMap{
	"inc":      func(n int) int { return n + 1 },
	"dec":      func(n int) int { return n - 1 },
	"truncate": truncate,
}

// Renderer compiles the templates once and renders them with a shared layout.
type Renderer struct {
	templates map[string]*template.Template
}

// New parses every page template against the shared layout.
func New() (*Renderer, error) {
	r := &Renderer{templates: make(map[string]*template.Template, len(pages))}

	for _, page := range pages {
		t, err := template.New("layout").Funcs(funcs).ParseFS(resources.Templates, "templates/layout.html", "templates/"+page)
		if err != nil {
			return nil, err
		}
		r.templates[page] = t
	}

	return r, nil
}

// Render writes the given page to w. It buffers first so a template error never
// produces a half-written response with an already-sent status code.
func (r *Renderer) Render(w http.ResponseWriter, page string, status int, data any) {
	t, ok := r.templates[page]
	if !ok {
		http.Error(w, "unknown template: "+page, http.StatusInternalServerError)
		return
	}

	var buf bytes.Buffer
	if err := t.ExecuteTemplate(&buf, "layout", data); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(status)
	_, _ = buf.WriteTo(w)
}

// truncate shortens s to at most n runes, appending an ellipsis when cut.
func truncate(n int, s string) string {
	runes := []rune(s)
	if len(runes) <= n {
		return s
	}

	return string(runes[:n]) + "…"
}
