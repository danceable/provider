package provider

import (
	"context"
	"time"
)

// options holds the configuration options for the service provider manager.
type options struct {
	// TerminationDelay is the duration to wait for providers to terminate gracefully before forcing termination.
	TerminationDelay time.Duration

	// Termination deadline
	TerminationDeadline time.Duration

	// Callback is a function that will be called with the context and container when the manager starts running. This can be used to perform any setup or initialization that needs to happen after the providers have been booted but before the manager starts waiting for termination signals.
	Callback func(ctx context.Context, container Container)
}

// DefaultOptions returns a new instance of options with default values.
func DefaultOptions() *options {
	return &options{
		TerminationDelay:    300 * time.Millisecond,
		TerminationDeadline: 200 * time.Millisecond,
		Callback:            nil,
	}
}

// Option is a function that configures the service provider manager.
type Option func(*options)

// WithTerminationDelay sets the duration to wait for providers to terminate gracefully before forcing termination.
func WithTerminationDelay(delay time.Duration) Option {
	return func(opts *options) {
		opts.TerminationDelay = delay
	}
}

// WithCallback sets the callback function that will be called with the context and container when the manager starts running.
func WithCallback(callback func(ctx context.Context, container Container)) Option {
	return func(opts *options) {
		opts.Callback = callback
	}
}

// WithTerminationDeadline sets the duration to wait for providers to terminate before forcing termination.
func WithTerminationDeadline(deadline time.Duration) Option {
	return func(opts *options) {
		opts.TerminationDeadline = deadline
	}
}

// scopeConfig collects the per-scope configuration produced by ScopeOptions.
type scopeConfig struct {
	// values are bound into the scoped container before its providers run.
	values []scopedValue

	// name is the scope name; only used when persistent is true.
	name string

	// persistent makes the scope a named, persistent child instead of an
	// anonymous, ephemeral one.
	persistent bool

	// autoTerminate ties the scope's teardown to the context: the scope is
	// terminated automatically once the context passed to Scope is cancelled.
	autoTerminate bool
}

// scopedValue is a single named value seeded into a scoped container.
type scopedValue struct {
	name  string
	value any
}

// ScopeOption configures a scoped instance of the container.
type ScopeOption func(*scopeConfig)

// newScopeConfig collects the options into the configuration of one scope.
func newScopeConfig(opts []ScopeOption) *scopeConfig {
	config := &scopeConfig{}
	for _, opt := range opts {
		opt(config)
	}

	return config
}

// WithValue seeds the scoped container with value, resolvable by name. The
// value is bound as a named singleton, so scoped providers (and anything else
// resolving from the scope) can retrieve it via resolve.WithName(name).
func WithValue(name string, value any) ScopeOption {
	return func(c *scopeConfig) {
		c.values = append(c.values, scopedValue{name: name, value: value})
	}
}

// WithPersistent makes the scope a named, persistent child of the manager's
// container (container.Scope) rather than the default anonymous, ephemeral one
// (container.Derive). The named child is cached on its parent and reused by
// later calls with the same name.
func WithPersistent(name string) ScopeOption {
	return func(c *scopeConfig) {
		c.persistent = true
		c.name = name
	}
}

// WithAutoTermination makes the scope terminate itself once the context passed
// to Scope is cancelled, releasing the caller from calling Terminate. Teardown
// runs exactly once, whether triggered by the context or by an explicit
// Terminate, so combining the two is safe.
func WithAutoTermination() ScopeOption {
	return func(c *scopeConfig) {
		c.autoTerminate = true
	}
}
