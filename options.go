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
