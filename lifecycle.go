package provider

import (
	"context"
	"slices"
	"time"
)

// The phases of the provider lifecycle, each expressed once over a registry. The
// manager runs them against its own providers and every scope runs them against
// its copy of the scoped ones, so global and scoped providers follow exactly the
// same rules — including a deferred provider, which runs the same two phases
// late, on its own.
//
// Each phase records what it got through, because that is what termination goes
// by: a provider is terminated once its Register has run, whether or not it went
// on to boot.

// register registers the providers that run at boot, walking past the deferred
// ones: those register when something asks for what they provide.
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

// boot boots the providers that run at boot.
func boot(ctx context.Context, providers *registry, container Container) error {
	for _, entry := range providers.list() {
		if entry.deferred() {
			continue
		}

		if err := entry.provider.Boot(ctx, container); err != nil {
			return err
		}

		providers.booted(entry)
	}

	return nil
}

// terminate terminates the providers that registered, in reverse order. A
// provider whose Register never ran holds nothing to release: a deferred one
// nothing ever asked for, or one the phases had not reached when they failed.
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

// load registers and boots a single deferred provider against the container it
// is given, recording each step exactly as the boot-time phases do.
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

// shutdown waits out the grace period, then terminates the providers within the
// termination deadline, reporting the deadline's error when they do not finish
// in time.
func shutdown(providers *registry, opts *options) error {
	// wait for a grace period to allow providers to terminate gracefully.
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
