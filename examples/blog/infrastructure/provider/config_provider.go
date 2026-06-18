// Package provider holds the service providers that wire the blog's
// dependency graph using github.com/danceable/provider. Each provider owns one
// concern (config, database, domain services, HTTP) and registers its
// bindings into the container; the manager boots and terminates them in order.
package provider

import (
	"context"

	"github.com/danceable/container/bind"
	"github.com/danceable/provider"
	"github.com/danceable/provider/examples/blog/infrastructure/config"
)

// ConfigProvider binds the application configuration loaded from the environment.
type ConfigProvider struct{}

// compile-time assertion that the type satisfies the provider contract.
var _ provider.Provider = (*ConfigProvider)(nil)

// NewConfigProvider creates a ConfigProvider.
func NewConfigProvider() *ConfigProvider { return &ConfigProvider{} }

// Order makes config the very first thing registered, since everything else
// depends on it.
func (p *ConfigProvider) Order() int { return 0 }

// Register binds *config.Config as a singleton.
func (p *ConfigProvider) Register(_ context.Context, c provider.Container) error {
	return c.Bind(func() (*config.Config, error) {
		return config.FromEnv()
	}, bind.Singleton())
}

// Boot has nothing to do; the binding is eager.
func (p *ConfigProvider) Boot(_ context.Context, _ provider.Container) error { return nil }

// Terminate has nothing to release.
func (p *ConfigProvider) Terminate(_ context.Context) error { return nil }
