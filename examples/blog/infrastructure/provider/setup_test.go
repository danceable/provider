package provider_test

import (
	"github.com/danceable/container"
	"github.com/danceable/provider"
	"github.com/danceable/provider/adapters/danceable"
)

// newTestContainer returns an isolated container for a single test, backed by a
// real danceable/container wrapped in the production danceable adapter.
func newTestContainer() provider.Container {
	return danceable.New(container.New())
}
