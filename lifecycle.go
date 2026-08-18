package provider

import (
	"context"
	"slices"
	"time"
)

// The phases of the provider lifecycle, each written once over a registry: the
// manager runs them against its own providers, every scope against its copy of
// the scoped ones, and a deferred provider runs the same two phases late.
//
// Each phase records what it got through, because termination goes by that.

func register(ctx context.Context, providers *registry, container Container) error {
	for _, entry := range providers.list() {
		if entry.deferred() {
			continue
		}

		if err := entry.provider.Register(ctx, container); err != nil {
			return err
		}

		entry.registered.Store(true)
	}

	return nil
}

func boot(ctx context.Context, providers *registry, container Container) error {
	for _, entry := range providers.list() {
		// A provider registered after the register phase walked past it never
		// bound anything, so it takes no part in this run.
		if entry.deferred() || !entry.registered.Load() {
			continue
		}

		if err := entry.provider.Boot(ctx, container); err != nil {
			return err
		}

		providers.booted(entry)
	}

	return nil
}

// load runs a deferred provider through both phases against the container it is
// given.
func load(ctx context.Context, providers *registry, entry *registration, container Container) error {
	if err := entry.provider.Register(ctx, container); err != nil {
		return err
	}

	entry.registered.Store(true)

	if err := entry.provider.Boot(ctx, container); err != nil {
		return err
	}

	providers.booted(entry)

	return nil
}

// terminate terminates the providers that registered, in reverse order. One
// whose Register never ran holds nothing to release.
func terminate(ctx context.Context, providers *registry) error {
	entries := providers.list()

	for i := range slices.Backward(entries) {
		entry := entries[i]
		if !entry.registered.Load() {
			continue
		}

		if err := entry.provider.Terminate(ctx); err != nil {
			return err
		}
	}

	return nil
}

// shutdown waits out the grace period, then terminates within the deadline.
func shutdown(providers *registry, opts *options) error {
	time.Sleep(opts.TerminationDelay)

	ctx, cancel := context.WithTimeout(context.Background(), opts.TerminationDeadline)
	defer cancel()

	terminated := make(chan error, 1)
	go func() { terminated <- terminate(ctx, providers) }()

	select {
	case <-ctx.Done():
		return ctx.Err()
	case err := <-terminated:
		return err
	}
}
