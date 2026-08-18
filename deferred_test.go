package provider

import (
	"context"
	"errors"
	"reflect"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/danceable/container/bind"
	"github.com/danceable/container/resolve"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// deferredProvider is a recordingProvider that also implements Deferrable.
type deferredProvider struct {
	recordingProvider
	provides []reflect.Type
}

func (p *deferredProvider) Provides() []reflect.Type { return p.provides }

// orderedDeferredProvider adds an explicit execution order.
type orderedDeferredProvider struct {
	deferredProvider
	order int
}

func (p *orderedDeferredProvider) Order() int { return p.order }

// bindShape returns a Register hook binding shape to a square of the given side.
func bindShape(side int) func(context.Context, Container) error {
	return func(_ context.Context, c Container) error {
		return c.Bind(func() shape { return &square{side: side} }, bind.Singleton())
	}
}

// runManager runs the manager to completion with an already-cancelled context,
// leaving the deferred providers to be triggered by the returned container.
func runManager(t *testing.T, m *Manager) Container {
	t.Helper()

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	booted := make(chan Container, 1)
	require.NoError(t, m.Run(ctx,
		WithTerminationDelay(0),
		WithCallback(func(_ context.Context, c Container) { booted <- c }),
	))

	select {
	case c := <-booted:
		return c
	case <-time.After(time.Second):
		t.Fatal("callback did not receive the container")
		return nil
	}
}

func TestDeferred_NotLoadedByRun(t *testing.T) {
	t.Parallel()

	m := newRealManager()

	var calls []string
	m.Register(&deferredProvider{
		recordingProvider: recordingProvider{name: "deferred", calls: &calls},
		provides:          []reflect.Type{reflect.TypeFor[shape]()},
	})
	m.Register(&recordingProvider{name: "eager", calls: &calls})

	runManager(t, m)

	// The eager provider ran its full lifecycle; the deferred one never ran.
	assert.Equal(t, []string{"eager.Register", "eager.Boot", "eager.Terminate"}, calls)
}

func TestDeferred_LoadedOnResolve(t *testing.T) {
	t.Parallel()

	m := newRealManager()

	var calls []string
	m.Register(&deferredProvider{
		recordingProvider: recordingProvider{name: "deferred", calls: &calls, onRegister: bindShape(4)},
		provides:          []reflect.Type{reflect.TypeFor[shape]()},
	})

	c := runManager(t, m)
	assert.Empty(t, calls)

	var s shape
	require.NoError(t, c.Resolve(&s))
	assert.Equal(t, 16, s.Area())
	assert.Equal(t, []string{"deferred.Register", "deferred.Boot"}, calls)
}

func TestDeferred_LoadedOnlyOnce(t *testing.T) {
	t.Parallel()

	m := newRealManager()

	var calls []string
	m.Register(&deferredProvider{
		recordingProvider: recordingProvider{name: "deferred", calls: &calls, onRegister: bindShape(2)},
		provides:          []reflect.Type{reflect.TypeFor[shape]()},
	})

	c := runManager(t, m)

	for range 3 {
		var s shape
		require.NoError(t, c.Resolve(&s))
	}

	assert.Equal(t, []string{"deferred.Register", "deferred.Boot"}, calls)
}

func TestDeferred_NotTriggeredByAnotherType(t *testing.T) {
	t.Parallel()

	m := newRealManager()

	var calls []string
	m.Register(&deferredProvider{
		recordingProvider: recordingProvider{name: "deferred", calls: &calls, onRegister: bindShape(2)},
		provides:          []reflect.Type{reflect.TypeFor[shape]()},
	})

	c := runManager(t, m)
	require.NoError(t, c.Bind(func() int { return 7 }, bind.Singleton()))

	var n int
	require.NoError(t, c.Resolve(&n))

	assert.Empty(t, calls)
}

func TestDeferred_LoadedOnCall(t *testing.T) {
	t.Parallel()

	m := newRealManager()

	var calls []string
	m.Register(&deferredProvider{
		recordingProvider: recordingProvider{
			name:  "deferred",
			calls: &calls,
			onRegister: func(_ context.Context, c Container) error {
				return c.Bind(func() *square { return &square{side: 5} }, bind.Singleton())
			},
		},
		// The provided concrete also triggers on the interfaces it implements,
		// mirroring the container's own lookup fallback.
		provides: []reflect.Type{reflect.TypeFor[*square]()},
	})

	c := runManager(t, m)

	var area int
	require.NoError(t, c.Call(func(s shape) { area = s.Area() }))

	assert.Equal(t, 25, area)
	assert.Equal(t, []string{"deferred.Register", "deferred.Boot"}, calls)
}

func TestDeferred_LoadedOnFill(t *testing.T) {
	t.Parallel()

	m := newRealManager()

	var calls []string
	m.Register(&deferredProvider{
		recordingProvider: recordingProvider{name: "deferred", calls: &calls, onRegister: bindShape(3)},
		provides:          []reflect.Type{reflect.TypeFor[shape]()},
	})

	c := runManager(t, m)

	var target struct {
		Shape shape `container:"type"`
	}
	require.NoError(t, c.Fill(&target))

	assert.Equal(t, 9, target.Shape.Area())
	assert.Equal(t, []string{"deferred.Register", "deferred.Boot"}, calls)
}

func TestDeferred_LoadsChainedProvider(t *testing.T) {
	t.Parallel()

	m := newRealManager()

	var calls []string

	// The shape provider needs an area, which a second deferred provider binds.
	m.Register(&deferredProvider{
		recordingProvider: recordingProvider{
			name:  "shape",
			calls: &calls,
			onRegister: func(_ context.Context, c Container) error {
				var area int
				if err := c.Resolve(&area, resolve.WithName("area")); err != nil {
					return err
				}

				return c.Bind(func() shape { return &circle{area: area} }, bind.Singleton())
			},
		},
		provides: []reflect.Type{reflect.TypeFor[shape]()},
	})

	m.Register(&deferredProvider{
		recordingProvider: recordingProvider{
			name:  "area",
			calls: &calls,
			onRegister: func(_ context.Context, c Container) error {
				return c.Bind(func() int { return 12 }, bind.WithName("area"), bind.Singleton())
			},
		},
		provides: []reflect.Type{reflect.TypeFor[int]()},
	})

	c := runManager(t, m)

	var s shape
	require.NoError(t, c.Resolve(&s))

	assert.Equal(t, 12, s.Area())
	assert.Equal(t, []string{
		"shape.Register", "area.Register", "area.Boot", "shape.Boot",
	}, calls)
}

func TestDeferred_ResolvesItsOwnTypeWhileBooting(t *testing.T) {
	t.Parallel()

	m := newRealManager()

	var booted shape
	m.Register(&deferredProvider{
		recordingProvider: recordingProvider{
			name:       "deferred",
			onRegister: bindShape(7),
			// Binding in Register and resolving in Boot is the common pattern:
			// the provider must not end up waiting for its own load.
			onBoot: func(_ context.Context, c Container) error {
				return c.Resolve(&booted)
			},
		},
		provides: []reflect.Type{reflect.TypeFor[shape]()},
	})

	c := runManager(t, m)

	var s shape
	require.NoError(t, c.Resolve(&s))

	require.NotNil(t, booted)
	assert.Equal(t, 49, booted.Area())
}

func TestDeferred_LoadedWithTheRunContext(t *testing.T) {
	t.Parallel()

	m := newRealManager()

	// The state of the load context, sampled while the provider is loading.
	loadCtxErr := make(chan error, 1)
	m.Register(&deferredProvider{
		recordingProvider: recordingProvider{
			name: "deferred",
			onRegister: func(ctx context.Context, c Container) error {
				loadCtxErr <- ctx.Err()
				return bindShape(1)(ctx, c)
			},
		},
		provides: []reflect.Type{reflect.TypeFor[shape]()},
	})

	ctx, cancel := context.WithCancel(context.Background())

	require.NoError(t, m.Run(ctx,
		WithTerminationDelay(0),
		WithCallback(func(_ context.Context, _ Container) {
			// Trigger the load from a scope whose context is already dead.
			dead, cancelScope := context.WithCancel(context.Background())
			cancelScope()

			scope, err := m.Scope(dead)
			assert.NoError(t, err)

			var s shape
			assert.NoError(t, scope.Container().Resolve(&s))

			cancel()
		}),
	))

	select {
	case err := <-loadCtxErr:
		// The provider lives as long as the application, so it is loaded with
		// Run's live context, not the dead one of the caller that triggered it.
		assert.NoError(t, err)
	case <-time.After(time.Second):
		t.Fatal("the deferred provider was not loaded")
	}
}

func TestDeferred_ConcurrentResolveWaitsForTheLoad(t *testing.T) {
	t.Parallel()

	m := newRealManager()

	release := make(chan struct{})
	m.Register(&deferredProvider{
		recordingProvider: recordingProvider{
			name: "deferred",
			onRegister: func(ctx context.Context, c Container) error {
				// Hold the load open so the other goroutines pile up behind it.
				<-release
				return bindShape(8)(ctx, c)
			},
		},
		provides: []reflect.Type{reflect.TypeFor[shape]()},
	})

	c := runManager(t, m)

	var wg sync.WaitGroup
	for range 10 {
		wg.Add(1)
		go func() {
			defer wg.Done()

			var s shape
			if assert.NoError(t, c.Resolve(&s)) {
				assert.Equal(t, 64, s.Area())
			}
		}()
	}

	close(release)
	wg.Wait()
}

func TestDeferred_LoadedInOrder(t *testing.T) {
	t.Parallel()

	m := newRealManager()

	var calls []string
	// Both are triggered by the same type, so the batch runs lowest order first.
	m.Register(&orderedDeferredProvider{
		deferredProvider: deferredProvider{
			recordingProvider: recordingProvider{name: "second", calls: &calls},
			provides:          []reflect.Type{reflect.TypeFor[shape]()},
		},
		order: 10,
	})
	m.Register(&orderedDeferredProvider{
		deferredProvider: deferredProvider{
			recordingProvider: recordingProvider{name: "first", calls: &calls, onRegister: bindShape(1)},
			provides:          []reflect.Type{reflect.TypeFor[shape]()},
		},
		order: 5,
	})

	c := runManager(t, m)

	var s shape
	require.NoError(t, c.Resolve(&s))

	assert.Equal(t, []string{
		"first.Register", "first.Boot", "second.Register", "second.Boot",
	}, calls)
}

func TestDeferred_EmptyProvidesRunsEagerly(t *testing.T) {
	t.Parallel()

	m := newRealManager()

	var calls []string
	m.Register(&deferredProvider{
		recordingProvider: recordingProvider{name: "p", calls: &calls},
		provides:          nil,
	})

	runManager(t, m)

	// Nothing could ever trigger it, so it takes part in the regular lifecycle.
	assert.Equal(t, []string{"p.Register", "p.Boot", "p.Terminate"}, calls)
}

func TestDeferred_RegisterErrorPropagatesFromResolve(t *testing.T) {
	t.Parallel()

	m := newRealManager()
	regErr := errors.New("register failed")

	var calls []string
	m.Register(&deferredProvider{
		recordingProvider: recordingProvider{name: "deferred", calls: &calls, regErr: regErr},
		provides:          []reflect.Type{reflect.TypeFor[shape]()},
	})

	c := runManager(t, m)

	var s shape
	require.ErrorIs(t, c.Resolve(&s), regErr)
	assert.NotContains(t, calls, "deferred.Boot")
}

func TestDeferred_RegisterErrorPropagatesFromCall(t *testing.T) {
	t.Parallel()

	m := newRealManager()
	regErr := errors.New("register failed")

	var calls []string
	m.Register(&deferredProvider{
		recordingProvider: recordingProvider{name: "deferred", calls: &calls, regErr: regErr},
		provides:          []reflect.Type{reflect.TypeFor[shape]()},
	})

	c := runManager(t, m)

	// The load is triggered by the argument the call needs, and its failure is
	// reported instead of the call going ahead without it.
	called := false
	require.ErrorIs(t, c.Call(func(shape) { called = true }), regErr)

	assert.False(t, called)
	assert.NotContains(t, calls, "deferred.Boot")
}

func TestDeferred_RegisterErrorPropagatesFromFill(t *testing.T) {
	t.Parallel()

	m := newRealManager()
	regErr := errors.New("register failed")

	m.Register(&deferredProvider{
		recordingProvider: recordingProvider{name: "deferred", regErr: regErr},
		provides:          []reflect.Type{reflect.TypeFor[shape]()},
	})

	c := runManager(t, m)

	var target struct {
		Shape shape `container:"type"`
	}
	require.ErrorIs(t, c.Fill(&target), regErr)

	assert.Nil(t, target.Shape)
}

func TestDeferred_BootErrorPropagatesFromResolve(t *testing.T) {
	t.Parallel()

	m := newRealManager()
	bootErr := errors.New("boot failed")

	m.Register(&deferredProvider{
		recordingProvider: recordingProvider{name: "deferred", onRegister: bindShape(1), bootErr: bootErr},
		provides:          []reflect.Type{reflect.TypeFor[shape]()},
	})

	c := runManager(t, m)

	var s shape
	require.ErrorIs(t, c.Resolve(&s), bootErr)
}

func TestDeferred_TerminatedInItsOrderSlot(t *testing.T) {
	t.Parallel()

	m := newRealManager()

	var calls []string
	m.Register(&orderedRecordingProvider{
		recordingProvider: recordingProvider{name: "first", calls: &calls},
		order:             1,
	})
	m.Register(&orderedDeferredProvider{
		deferredProvider: deferredProvider{
			recordingProvider: recordingProvider{name: "deferred", calls: &calls, onRegister: bindShape(1)},
			provides:          []reflect.Type{reflect.TypeFor[shape]()},
		},
		order: 2,
	})
	m.Register(&orderedRecordingProvider{
		recordingProvider: recordingProvider{name: "last", calls: &calls},
		order:             3,
	})

	ctx, cancel := context.WithCancel(context.Background())

	require.NoError(t, m.Run(ctx,
		WithTerminationDelay(0),
		WithCallback(func(_ context.Context, c Container) {
			var s shape
			assert.NoError(t, c.Resolve(&s))
			cancel()
		}),
	))

	// It booted last, when the request came in, but it terminates in the slot its
	// order gives it: deferral changes when a provider runs, not where it sits.
	assert.Equal(t, []string{
		"first.Register",
		"last.Register",
		"first.Boot",
		"last.Boot",
		"deferred.Register",
		"deferred.Boot",
		"last.Terminate",
		"deferred.Terminate",
		"first.Terminate",
	}, calls)
}

func TestDeferred_NotTerminatedWhenNeverLoaded(t *testing.T) {
	t.Parallel()

	m := newRealManager()

	var calls []string
	m.Register(&recordingProvider{name: "eager", calls: &calls})
	m.Register(&deferredProvider{
		recordingProvider: recordingProvider{name: "deferred", calls: &calls},
		provides:          []reflect.Type{reflect.TypeFor[shape]()},
	})

	runManager(t, m)

	// The deferred provider never registered, so it has nothing to release.
	assert.Equal(t, []string{"eager.Register", "eager.Boot", "eager.Terminate"}, calls)
}

func TestDeferred_TerminatedWhenItRegisteredButFailedToBoot(t *testing.T) {
	t.Parallel()

	m := newRealManager()

	var calls []string
	m.Register(&deferredProvider{
		recordingProvider: recordingProvider{
			name:       "deferred",
			calls:      &calls,
			onRegister: bindShape(1),
			bootErr:    errors.New("boot failed"),
		},
		provides: []reflect.Type{reflect.TypeFor[shape]()},
	})

	ctx, cancel := context.WithCancel(context.Background())

	require.NoError(t, m.Run(ctx,
		WithTerminationDelay(0),
		WithCallback(func(_ context.Context, c Container) {
			var s shape
			assert.Error(t, c.Resolve(&s))
			cancel()
		}),
	))

	// Its Register ran, so whatever it took there is released at shutdown even
	// though it never finished booting.
	assert.Equal(t, []string{
		"deferred.Register",
		"deferred.Boot",
		"deferred.Terminate",
	}, calls)
}

func TestScopedDeferred_TerminatedWhenItRegisteredButFailedToBoot(t *testing.T) {
	t.Parallel()

	m := newRealManager()

	var calls []string
	m.Register(&deferredProvider{
		recordingProvider: recordingProvider{
			name:       "scoped",
			scoped:     true,
			calls:      &calls,
			onRegister: bindShape(1),
			bootErr:    errors.New("boot failed"),
		},
		provides: []reflect.Type{reflect.TypeFor[shape]()},
	})

	scope, err := m.Scope(context.Background())
	require.NoError(t, err)

	var s shape
	require.Error(t, scope.Container().Resolve(&s))
	require.NoError(t, scope.Terminate(context.Background()))

	assert.Equal(t, []string{"scoped.Register", "scoped.Boot", "scoped.Terminate"}, calls)
}

// TestDeferred_ConcurrentResolveLoadsOnce exercises the claim under lock: many
// goroutines resolve the same deferred type at once. Run with -race.
func TestDeferred_ConcurrentResolveLoadsOnce(t *testing.T) {
	t.Parallel()

	m := newRealManager()

	var registrations atomic.Int32
	m.Register(&deferredProvider{
		recordingProvider: recordingProvider{
			name: "deferred",
			onRegister: func(_ context.Context, c Container) error {
				registrations.Add(1)
				return c.Bind(func() shape { return &square{side: 2} }, bind.Singleton())
			},
		},
		provides: []reflect.Type{reflect.TypeFor[shape]()},
	})

	c := runManager(t, m)

	var wg sync.WaitGroup
	for range 50 {
		wg.Add(1)
		go func() {
			defer wg.Done()

			var s shape
			assert.NoError(t, c.Resolve(&s))
		}()
	}
	wg.Wait()

	assert.Equal(t, int32(1), registrations.Load())
}

func TestDeferred_NoDecorationWithoutDeferredProviders(t *testing.T) {
	t.Parallel()

	root := newAdapter(nil)
	m := New(root)

	// Nothing is deferred, so providers keep receiving the manager's container.
	var seen Container
	m.Register(&recordingProvider{
		name:       "p",
		onRegister: func(_ context.Context, c Container) error { seen = c; return nil },
	})

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	require.NoError(t, m.Run(ctx, WithTerminationDelay(0)))
	assert.Same(t, root, seen)
}

func TestScopedDeferred_LoadedPerScope(t *testing.T) {
	t.Parallel()

	m := newRealManager()

	var calls []string
	m.Register(&deferredProvider{
		recordingProvider: recordingProvider{
			name:       "scoped",
			scoped:     true,
			calls:      &calls,
			onRegister: bindShape(3),
		},
		provides: []reflect.Type{reflect.TypeFor[shape]()},
	})

	// Opening a scope does not load it.
	first, err := m.Scope(context.Background())
	require.NoError(t, err)
	assert.Empty(t, calls)

	// Resolving from the scope does.
	var s shape
	require.NoError(t, first.Container().Resolve(&s))
	assert.Equal(t, 9, s.Area())
	assert.Equal(t, []string{"scoped.Register", "scoped.Boot"}, calls)

	// A second scope loads its own copy, independently of the first.
	second, err := m.Scope(context.Background())
	require.NoError(t, err)
	require.NoError(t, second.Container().Resolve(&s))

	assert.Equal(t, []string{
		"scoped.Register", "scoped.Boot",
		"scoped.Register", "scoped.Boot",
	}, calls)

	require.NoError(t, first.Terminate(context.Background()))
	require.NoError(t, second.Terminate(context.Background()))

	assert.Equal(t, []string{
		"scoped.Register", "scoped.Boot",
		"scoped.Register", "scoped.Boot",
		"scoped.Terminate", "scoped.Terminate",
	}, calls)
}

func TestScopedDeferred_NotTerminatedWhenNeverLoaded(t *testing.T) {
	t.Parallel()

	m := newRealManager()

	var calls []string
	m.Register(&deferredProvider{
		recordingProvider: recordingProvider{name: "scoped", scoped: true, calls: &calls},
		provides:          []reflect.Type{reflect.TypeFor[shape]()},
	})

	scope, err := m.Scope(context.Background())
	require.NoError(t, err)
	require.NoError(t, scope.Terminate(context.Background()))

	assert.Empty(t, calls)
}

func TestScopedDeferred_TerminatedInItsOrderSlot(t *testing.T) {
	t.Parallel()

	m := newRealManager()

	var calls []string
	m.Register(&orderedRecordingProvider{
		recordingProvider: recordingProvider{name: "eager", scoped: true, calls: &calls},
		order:             1,
	})
	m.Register(&orderedDeferredProvider{
		deferredProvider: deferredProvider{
			recordingProvider: recordingProvider{name: "deferred", scoped: true, calls: &calls, onRegister: bindShape(1)},
			provides:          []reflect.Type{reflect.TypeFor[shape]()},
		},
		order: 2,
	})

	scope, err := m.Scope(context.Background())
	require.NoError(t, err)

	var s shape
	require.NoError(t, scope.Container().Resolve(&s))
	require.NoError(t, scope.Terminate(context.Background()))

	assert.Equal(t, []string{
		"eager.Register",
		"eager.Boot",
		"deferred.Register",
		"deferred.Boot",
		"deferred.Terminate",
		"eager.Terminate",
	}, calls)
}

func TestScopedDeferred_LoadedByEagerScopedProvider(t *testing.T) {
	t.Parallel()

	m := newRealManager()

	var calls []string
	m.Register(&orderedRecordingProvider{
		recordingProvider: recordingProvider{
			name:   "consumer",
			scoped: true,
			calls:  &calls,
			onBoot: func(_ context.Context, c Container) error {
				var s shape
				return c.Resolve(&s)
			},
		},
		order: 2,
	})
	m.Register(&deferredProvider{
		recordingProvider: recordingProvider{name: "deferred", scoped: true, calls: &calls, onRegister: bindShape(1)},
		provides:          []reflect.Type{reflect.TypeFor[shape]()},
	})

	scope, err := m.Scope(context.Background())
	require.NoError(t, err)
	t.Cleanup(func() { _ = scope.Terminate(context.Background()) })

	assert.Equal(t, []string{
		"consumer.Register",
		"consumer.Boot",
		"deferred.Register",
		"deferred.Boot",
	}, calls)
}

func TestDeferred_GlobalProviderLoadedFromScope(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		name string
		opts []ScopeOption
	}{
		{name: "ephemeral"},
		{name: "persistent", opts: []ScopeOption{WithPersistent("request")}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			m := newRealManager()

			var calls []string
			m.Register(&deferredProvider{
				recordingProvider: recordingProvider{name: "global", calls: &calls, onRegister: bindShape(6)},
				provides:          []reflect.Type{reflect.TypeFor[shape]()},
			})

			scope, err := m.Scope(context.Background(), tc.opts...)
			require.NoError(t, err)
			t.Cleanup(func() { _ = scope.Terminate(context.Background()) })

			// The scope's container inherits the decoration of its parent, so a
			// globally deferred provider is loaded through it too.
			var s shape
			require.NoError(t, scope.Container().Resolve(&s))

			assert.Equal(t, 36, s.Area())
			assert.Equal(t, []string{"global.Register", "global.Boot"}, calls)
		})
	}
}

func TestDeferred_InvalidRequestsReachTheContainer(t *testing.T) {
	t.Parallel()

	m := newRealManager()

	var calls []string
	m.Register(&deferredProvider{
		recordingProvider: recordingProvider{name: "deferred", calls: &calls},
		provides:          []reflect.Type{reflect.TypeFor[shape]()},
	})

	c := runManager(t, m)

	// Requests the container itself rejects trigger nothing; the container
	// stays the single source of those errors.
	var s shape
	assert.Error(t, c.Resolve(s))
	assert.Error(t, c.Call("not a function"))
	assert.Error(t, c.Fill(&s))
	assert.Empty(t, calls)
}
