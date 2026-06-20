package provider

import (
	"context"

	"github.com/danceable/container/bind"
	"github.com/danceable/provider"
	"github.com/danceable/provider/examples/blog/infrastructure/i18n"
	"github.com/danceable/provider/examples/blog/infrastructure/repositories/memory"
	"github.com/danceable/provider/examples/blog/presenation/http/middlewares"
)

// I18nProvider binds the application-wide internationalization dependencies: the
// hard-coded translation repository and the scope opener that the HTTP layer
// uses to build a request-scoped Translator.
type I18nProvider struct{}

var _ provider.Provider = (*I18nProvider)(nil)

// NewI18nProvider creates an I18nProvider.
func NewI18nProvider() *I18nProvider { return &I18nProvider{} }

// Order registers i18n after config but before the domain and HTTP providers.
func (p *I18nProvider) Order() int { return 5 }

// Register binds the translation repository and the scope opener.
func (p *I18nProvider) Register(_ context.Context, c provider.Container) error {
	if err := c.Bind(func() i18n.Repository {
		return memory.NewTranslationRepository()
	}, bind.Singleton()); err != nil {
		return err
	}

	// The HTTP middleware opens a scope per request; the running manager is the
	// scope opener that drives the scoped TranslatorProvider.
	return c.Bind(func() middlewares.Scoper {
		return provider.Default
	}, bind.Singleton())
}

// Boot has nothing to do; the bindings are eager.
func (p *I18nProvider) Boot(_ context.Context, _ provider.Container) error { return nil }

// Terminate has nothing to release.
func (p *I18nProvider) Terminate(_ context.Context) error { return nil }
