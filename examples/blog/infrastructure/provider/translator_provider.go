package provider

import (
	"context"

	"github.com/danceable/container/bind"
	"github.com/danceable/container/resolve"
	"github.com/danceable/provider"
	"github.com/danceable/provider/examples/blog/infrastructure/i18n"
)

// TranslatorProvider is a scoped provider: it runs each time the HTTP layer
// opens a request scope and binds a *i18n.Translator for the language seeded
// into that scope (under i18n.LanguageValue). Because it implements
// provider.HasScope, Register routes it to the scoped set automatically.
type TranslatorProvider struct{}

var _ provider.Provider = (*TranslatorProvider)(nil)

// NewTranslatorProvider creates a TranslatorProvider.
func NewTranslatorProvider() *TranslatorProvider { return &TranslatorProvider{} }

// Scoped marks the provider for per-scope execution.
func (p *TranslatorProvider) Scoped() bool { return true }

// Register binds a Translator built from the shared repository (resolved from an
// ancestor scope) and the language seeded into this scope.
func (p *TranslatorProvider) Register(_ context.Context, c provider.Container) error {
	return c.Bind(func(repo i18n.Repository) (*i18n.Translator, error) {
		var lang i18n.Language
		if err := c.Resolve(&lang, resolve.WithName(i18n.LanguageValue)); err != nil {
			return nil, err
		}

		return i18n.NewTranslator(repo, lang), nil
	}, bind.Singleton(), bind.Lazy())
}

// Boot has nothing to do.
func (p *TranslatorProvider) Boot(_ context.Context, _ provider.Container) error { return nil }

// Terminate has nothing to release.
func (p *TranslatorProvider) Terminate(_ context.Context) error { return nil }
