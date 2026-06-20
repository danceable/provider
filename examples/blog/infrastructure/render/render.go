// Package render is the HTML rendering infrastructure: it compiles the embedded
// templates once and renders page+layout pairs to an http.ResponseWriter. It is
// a technical adapter, so the presentation layer depends on it rather than on
// html/template directly.
//
// Templates are language-agnostic: every visible string is a translation key
// looked up through the {{ t }} template function, whose implementation is bound
// per request from the Translator in the request context.
package render

import (
	"bytes"
	"html/template"
	"net/http"

	"github.com/danceable/provider/examples/blog/infrastructure/i18n"
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

// funcs are the helpers available inside every template. The i18n helpers (t,
// lang, dir) are placeholders here so the templates parse; Render overrides them
// per request with implementations bound to the request's Translator.
var funcs = template.FuncMap{
	"inc":      func(n int) int { return n + 1 },
	"dec":      func(n int) int { return n - 1 },
	"truncate": truncate,

	"t":    func(key string) string { return key },
	"lang": func() string { return string(i18n.Default) },
	"dir":  func() string { return "ltr" },

	"languages": func() []i18n.Language { return i18n.Supported },
	"native":    func(l i18n.Language) string { return i18n.Native[l] },
}

// Renderer compiles the templates once and renders them with a shared layout.
type Renderer struct {
	templates map[string]*template.Template

	// defaultT translates pages when a request carries no Translator (for
	// example in tests that exercise a handler without the i18n middleware).
	defaultT *i18n.Translator
}

// New parses every page template against the shared layout. defaultT is the
// fallback translator used when a request has none in its context.
func New(defaultT *i18n.Translator) (*Renderer, error) {
	r := &Renderer{
		templates: make(map[string]*template.Template, len(pages)),
		defaultT:  defaultT,
	}

	for _, page := range pages {
		t, err := template.New("layout").Funcs(funcs).ParseFS(resources.Templates, "templates/layout.html", "templates/"+page)
		if err != nil {
			return nil, err
		}
		r.templates[page] = t
	}

	return r, nil
}

// Render writes the given page to w, translated for the request's language. It
// buffers first so a template error never produces a half-written response with
// an already-sent status code.
func (r *Renderer) Render(w http.ResponseWriter, req *http.Request, page string, status int, data any) {
	base, ok := r.templates[page]
	if !ok {
		http.Error(w, "unknown template: "+page, http.StatusInternalServerError)
		return
	}

	// Resolve the request's translator, falling back to the renderer default.
	translator := i18n.FromContext(req.Context())
	if translator == nil {
		translator = r.defaultT
	}

	// Clone so the per-request i18n functions don't race across requests, then
	// override the parse-time placeholders with this request's translator.
	t, err := base.Clone()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	t.Funcs(i18nFuncs(translator))

	var buf bytes.Buffer
	if err := t.ExecuteTemplate(&buf, "layout", data); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(status)
	_, _ = buf.WriteTo(w)
}

// i18nFuncs returns the t/lang/dir helpers bound to translator. A nil translator
// degrades to identity helpers so a misconfigured request still renders.
func i18nFuncs(translator *i18n.Translator) template.FuncMap {
	if translator == nil {
		return template.FuncMap{
			"t":    func(key string) string { return key },
			"lang": func() string { return string(i18n.Default) },
			"dir":  func() string { return "ltr" },
		}
	}

	return template.FuncMap{
		"t":    translator.T,
		"lang": func() string { return string(translator.Lang()) },
		"dir":  translator.Dir,
	}
}

// truncate shortens s to at most n runes, appending an ellipsis when cut.
func truncate(n int, s string) string {
	runes := []rune(s)
	if len(runes) <= n {
		return s
	}

	return string(runes[:n]) + "…"
}
