package provider

import (
	"context"

	"github.com/danceable/container"
	"github.com/danceable/provider/adapters/danceable"
)

// Default is the default service provider manager. It is backed by the
// danceable/container global container, wrapped in the danceable adapter. To use
// a different backend (for example uber/dig), build a manager explicitly with
// New and the corresponding adapter.
var Default = New(danceable.New(container.Default))

// Register calls the Register method of the default service provider manager.
func Register(provider Provider) {
	Default.Register(provider)
}

// Run calls the Run method of the default service provider manager.
func Run(ctx context.Context, opts ...Option) error {
	return Default.Run(ctx, opts...)
}
