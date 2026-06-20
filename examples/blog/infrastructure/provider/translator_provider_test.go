package provider_test

import (
	"context"
	"testing"

	"github.com/danceable/provider"
	"github.com/danceable/provider/examples/blog/infrastructure/i18n"
	blogprovider "github.com/danceable/provider/examples/blog/infrastructure/provider"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestTranslatorProvider_ScopedPerLanguage proves the scoped-provider flow end
// to end: the global I18nProvider binds the translation repository once, and the
// scoped TranslatorProvider yields a Translator for whichever language each
// request scope is seeded with via provider.WithValue.
//
// It drives the global manager because a real container-backed *provider.Manager
// is only reachable through provider.Default; the test therefore does not run in
// parallel.
func TestTranslatorProvider_ScopedPerLanguage(t *testing.T) {
	m := provider.Default

	m.Register(blogprovider.NewI18nProvider())       // global: binds the repository
	m.Register(blogprovider.NewTranslatorProvider()) // scoped: runs per Scope

	// Run with an already-cancelled context: register+boot bind the global
	// repository, then Run returns straight away.
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	require.NoError(t, m.Run(ctx, provider.WithTerminationDelay(0)))

	want := map[i18n.Language]string{
		i18n.English: "Home",
		i18n.German:  "Startseite",
		i18n.Persian: "خانه",
		i18n.Chinese: "首页",
	}

	for lang, home := range want {
		scope, err := m.Scope(context.Background(), provider.WithValue(i18n.LanguageValue, lang))
		require.NoError(t, err)

		var translator *i18n.Translator
		require.NoError(t, scope.Container().Resolve(&translator))

		assert.Equal(t, lang, translator.Lang())
		assert.Equal(t, home, translator.T("nav.home"))

		require.NoError(t, scope.Terminate(context.Background()))
	}
}
