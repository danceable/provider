package provider

import (
	"context"

	"github.com/danceable/container"
)

// Default is the default concrete of the service provider manager.
var Default = New(container.Default)

// Register calls the Register method of the default service provider manager.
func Register(provider Provider) {
	Default.Register(provider)
}

// Run calls the Run method of the default service provider manager.
func Run(ctx context.Context) error {
	return Default.Run(ctx)
}
