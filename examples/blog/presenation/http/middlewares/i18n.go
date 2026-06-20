// Package middlewares holds the HTTP middlewares of the presentation layer.
package middlewares

import (
	"context"
	"net/http"

	"github.com/danceable/provider"
	"github.com/danceable/provider/examples/blog/infrastructure/i18n"
)

// Scoper opens a scoped instance of the DI container. *provider.Manager
// satisfies it; the HTTP layer uses it to obtain a request-scoped Translator
// without depending on the manager concretely.
type Scoper interface {
	Scope(ctx context.Context, opts ...provider.ScopeOption) (*provider.Scope, error)
}

// WithI18n wraps next so every request carries a Translator for its language.
//
// For each request it detects the language, opens a request scope seeded with
// it (provider.WithValue), lets the scoped TranslatorProvider build a Translator
// for that language, and stores it in the request context for the renderer. The
// scope is terminated when the request returns. If the scope cannot be opened or
// resolved, the request proceeds and the renderer falls back to its default
// language, so i18n never takes the site down.
func WithI18n(scoper Scoper, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		lang := i18n.Detect(r)

		// Persist an explicit ?lang choice so it survives later requests.
		if _, explicit := i18n.Parse(r.URL.Query().Get("lang")); explicit {
			i18n.SetCookie(w, lang)
		}

		scope, err := scoper.Scope(r.Context(), provider.WithValue(i18n.LanguageValue, lang))
		if err != nil {
			next.ServeHTTP(w, r)
			return
		}
		defer func() { _ = scope.Terminate(r.Context()) }()

		var translator *i18n.Translator
		if err := scope.Container().Resolve(&translator); err != nil {
			next.ServeHTTP(w, r)
			return
		}

		next.ServeHTTP(w, r.WithContext(i18n.WithTranslator(r.Context(), translator)))
	})
}
