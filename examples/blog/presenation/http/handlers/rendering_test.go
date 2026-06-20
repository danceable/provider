package handlers_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/danceable/provider/examples/blog/infrastructure/i18n"
	"github.com/danceable/provider/examples/blog/infrastructure/repositories/memory"
	"github.com/danceable/provider/examples/blog/presenation/http/handlers"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// withTranslator injects tr into the request context, standing in for the i18n
// middleware so a handler can be exercised in a specific language.
func withTranslator(tr *i18n.Translator, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		next.ServeHTTP(w, r.WithContext(i18n.WithTranslator(r.Context(), tr)))
	})
}

func TestHome_RendersInRequestLanguage(t *testing.T) {
	t.Parallel()

	h := handlers.NewPublic(newService(), newRenderer(t), 5)
	persian := i18n.NewTranslator(memory.NewTranslationRepository(), i18n.Persian)

	srv := httptest.NewServer(withTranslator(persian, http.HandlerFunc(h.Home)))
	t.Cleanup(srv.Close)

	res, err := http.Get(srv.URL + "/")
	require.NoError(t, err)
	defer res.Body.Close()

	body := readBody(t, res)
	assert.Contains(t, body, "تازه‌ترین نوشته‌ها") // home.heading, Persian
	assert.Contains(t, body, `lang="fa"`)
	assert.Contains(t, body, `dir="rtl"`)
}

func TestHome_DefaultsToEnglishWithoutMiddleware(t *testing.T) {
	t.Parallel()

	srv, _ := newPublicServer(t)

	res, err := http.Get(srv.URL + "/")
	require.NoError(t, err)
	defer res.Body.Close()

	body := readBody(t, res)
	assert.Contains(t, body, "Latest articles")
	assert.Contains(t, body, `lang="en"`)
	assert.Contains(t, body, `dir="ltr"`)
}
