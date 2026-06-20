package i18n

import (
	"context"
	"net/http"
	"strings"
)

// LanguageValue is the name under which the requested Language is seeded into a
// request scope (via provider.WithValue) for the scoped Translator provider to
// read back.
const LanguageValue = "i18n.language"

// cookieName persists the visitor's language choice across requests.
const cookieName = "lang"

// ctxKey is the unexported context key under which the request Translator is stored.
type ctxKey struct{}

// WithTranslator returns a copy of ctx carrying t.
func WithTranslator(ctx context.Context, t *Translator) context.Context {
	return context.WithValue(ctx, ctxKey{}, t)
}

// FromContext returns the Translator stored in ctx, or nil if there is none.
func FromContext(ctx context.Context) *Translator {
	t, _ := ctx.Value(ctxKey{}).(*Translator)
	return t
}

// Detect resolves the request's language from, in order: the ?lang query
// parameter, the language cookie, the Accept-Language header, and finally the
// default language.
func Detect(r *http.Request) Language {
	if lang, ok := Parse(r.URL.Query().Get("lang")); ok {
		return lang
	}

	if c, err := r.Cookie(cookieName); err == nil {
		if lang, ok := Parse(c.Value); ok {
			return lang
		}
	}

	if lang, ok := fromAcceptLanguage(r.Header.Get("Accept-Language")); ok {
		return lang
	}

	return Default
}

// SetCookie persists lang as the visitor's choice for subsequent requests.
func SetCookie(w http.ResponseWriter, lang Language) {
	http.SetCookie(w, &http.Cookie{
		Name:     cookieName,
		Value:    string(lang),
		Path:     "/",
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
	})
}

// fromAcceptLanguage returns the first supported language listed in an
// Accept-Language header value, matching on the primary subtag (e.g. "de-DE" →
// "de") and ignoring quality weights.
func fromAcceptLanguage(header string) (Language, bool) {
	for _, part := range strings.Split(header, ",") {
		tag := part
		if i := strings.IndexByte(tag, ';'); i >= 0 {
			tag = tag[:i]
		}

		tag = strings.TrimSpace(tag)
		if i := strings.IndexByte(tag, '-'); i >= 0 {
			tag = tag[:i]
		}

		if lang, ok := Parse(tag); ok {
			return lang, true
		}
	}

	return "", false
}
