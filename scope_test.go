package provider

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/danceable/container"
	"github.com/danceable/container/bind"
	"github.com/danceable/container/resolve"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// recordingProvider records its lifecycle calls and can inject behavior or errors.
// It implements HasScope so the scoped flag decides how Register routes it.
type recordingProvider struct {
	name        string
	scoped      bool
	calls       *[]string
	onRegister  func(ctx context.Context, c Container) error
	onBoot      func(ctx context.Context, c Container) error
	onTerminate func(ctx context.Context) error
	regErr      error
	bootErr     error
	termErr     error
}

func (p *recordingProvider) Scoped() bool { return p.scoped }

func (p *recordingProvider) Register(ctx context.Context, c Container) error {
	if p.calls != nil {
		*p.calls = append(*p.calls, p.name+".Register")
	}
	if p.onRegister != nil {
		if err := p.onRegister(ctx, c); err != nil {
			return err
		}
	}
	return p.regErr
}

func (p *recordingProvider) Boot(ctx context.Context, c Container) error {
	if p.calls != nil {
		*p.calls = append(*p.calls, p.name+".Boot")
	}
	if p.onBoot != nil {
		if err := p.onBoot(ctx, c); err != nil {
			return err
		}
	}
	return p.bootErr
}

func (p *recordingProvider) Terminate(ctx context.Context) error {
	if p.calls != nil {
		*p.calls = append(*p.calls, p.name+".Terminate")
	}
	if p.onTerminate != nil {
		if err := p.onTerminate(ctx); err != nil {
			return err
		}
	}
	return p.termErr
}

// orderedRecordingProvider adds an explicit execution order.
type orderedRecordingProvider struct {
	recordingProvider
	order int
}

func (p *orderedRecordingProvider) Order() int { return p.order }

// shape mirrors the abstraction used by the adapter tests.
type shape interface{ Area() int }

type square struct{ side int }

func (s *square) Area() int { return s.side * s.side }

func newRealManager() *Manager {
	return New(newAdapter(container.New()))
}

func TestRegister_ScopedWithoutOrder(t *testing.T) {
	t.Parallel()

	m := newRealManager()

	var calls []string
	for i := 0; i < 3; i++ {
		m.Register(&recordingProvider{name: "p", scoped: true, calls: &calls})
	}

	scope, err := m.Scope(context.Background())
	require.NoError(t, err)
	require.NoError(t, scope.Terminate(context.Background()))

	assert.Equal(t, []string{
		"p.Register", "p.Register", "p.Register",
		"p.Boot", "p.Boot", "p.Boot",
		"p.Terminate", "p.Terminate", "p.Terminate",
	}, calls)
}

func TestScope_RunsScopedProvidersInOrder(t *testing.T) {
	t.Parallel()

	m := newRealManager()

	var calls []string
	p1 := &orderedRecordingProvider{recordingProvider: recordingProvider{name: "p1", scoped: true, calls: &calls}, order: 10}
	p2 := &orderedRecordingProvider{recordingProvider: recordingProvider{name: "p2", scoped: true, calls: &calls}, order: 5}
	p3 := &orderedRecordingProvider{recordingProvider: recordingProvider{name: "p3", scoped: true, calls: &calls}, order: 10}

	m.Register(p1)
	m.Register(p2)
	m.Register(p3)

	scope, err := m.Scope(context.Background(), WithPersistent("request"))
	require.NoError(t, err)
	require.NoError(t, scope.Terminate(context.Background()))

	// Lowest order first for Register/Boot, reverse for Terminate.
	assert.Equal(t, []string{
		"p2.Register", "p1.Register", "p3.Register",
		"p2.Boot", "p1.Boot", "p3.Boot",
		"p3.Terminate", "p1.Terminate", "p2.Terminate",
	}, calls)
}

func TestScope_PersistentAndEphemeralName(t *testing.T) {
	t.Parallel()

	m := newRealManager()

	persistent, err := m.Scope(context.Background(), WithPersistent("request"))
	require.NoError(t, err)
	assert.Equal(t, "request", persistent.Name())

	ephemeral, err := m.Scope(context.Background())
	require.NoError(t, err)
	assert.Empty(t, ephemeral.Name())
}

func TestWithValue_SeedsScopedContainer(t *testing.T) {
	t.Parallel()

	m := newRealManager()

	scope, err := m.Scope(context.Background(),
		WithValue("userID", 42),
		WithValue("region", "eu"),
	)
	require.NoError(t, err)

	var userID int
	require.NoError(t, scope.Container().Resolve(&userID, resolve.WithName("userID")))
	assert.Equal(t, 42, userID)

	var region string
	require.NoError(t, scope.Container().Resolve(&region, resolve.WithName("region")))
	assert.Equal(t, "eu", region)
}

func TestWithValue_ResolvableByScopedProvider(t *testing.T) {
	t.Parallel()

	m := newRealManager()

	var seen int
	m.Register(&recordingProvider{
		name:   "consumer",
		scoped: true,
		onBoot: func(_ context.Context, c Container) error {
			return c.Resolve(&seen, resolve.WithName("userID"))
		},
	})

	scope, err := m.Scope(context.Background(), WithPersistent("request"), WithValue("userID", 7))
	require.NoError(t, err)
	t.Cleanup(func() { _ = scope.Terminate(context.Background()) })

	// The value was bound before providers ran, so Boot could resolve it.
	assert.Equal(t, 7, seen)
}

func TestWithValue_NilValue(t *testing.T) {
	t.Parallel()

	m := newRealManager()

	scope, err := m.Scope(context.Background(), WithValue("nothing", nil))
	require.ErrorIs(t, err, ErrNilScopeValue)
	assert.Nil(t, scope)
}

func TestScope_InheritsParentBindings(t *testing.T) {
	t.Parallel()

	root := newAdapter(container.New())
	require.NoError(t, root.Bind(func() shape { return &square{side: 4} }, bind.Singleton()))

	m := New(root)

	scope, err := m.Scope(context.Background(), WithPersistent("request"))
	require.NoError(t, err)

	var s shape
	require.NoError(t, scope.Container().Resolve(&s))
	assert.Equal(t, 16, s.Area())
}

func TestScope_EphemeralScopesAreIndependent(t *testing.T) {
	t.Parallel()

	m := newRealManager()

	first, err := m.Scope(context.Background(), WithValue("id", "a"))
	require.NoError(t, err)
	second, err := m.Scope(context.Background(), WithValue("id", "b"))
	require.NoError(t, err)

	var firstID, secondID string
	require.NoError(t, first.Container().Resolve(&firstID, resolve.WithName("id")))
	require.NoError(t, second.Container().Resolve(&secondID, resolve.WithName("id")))

	assert.Equal(t, "a", firstID)
	assert.Equal(t, "b", secondID)
}

func TestWithPersistent_ReusesNamedChild(t *testing.T) {
	t.Parallel()

	m := newRealManager()

	first, err := m.Scope(context.Background(), WithPersistent("shared"))
	require.NoError(t, err)
	require.NoError(t, first.Container().Bind(func() shape { return &square{side: 3} }, bind.Singleton()))

	// A second persistent scope with the same name addresses the same child,
	// so it sees the binding registered through the first.
	second, err := m.Scope(context.Background(), WithPersistent("shared"))
	require.NoError(t, err)

	var s shape
	require.NoError(t, second.Container().Resolve(&s))
	assert.Equal(t, 9, s.Area())
}

func TestScope_RegisterErrorPropagates(t *testing.T) {
	t.Parallel()

	m := newRealManager()
	regErr := errors.New("register failed")

	var calls []string
	m.Register(&recordingProvider{name: "p", scoped: true, calls: &calls, regErr: regErr})

	scope, err := m.Scope(context.Background())
	require.ErrorIs(t, err, regErr)
	assert.Nil(t, scope)
	assert.NotContains(t, calls, "p.Boot")
}

func TestScope_BootErrorPropagates(t *testing.T) {
	t.Parallel()

	m := newRealManager()
	bootErr := errors.New("boot failed")

	m.Register(&recordingProvider{name: "p", scoped: true, bootErr: bootErr})

	scope, err := m.Scope(context.Background(), WithPersistent("request"))
	require.ErrorIs(t, err, bootErr)
	assert.Nil(t, scope)
}

func TestScope_TerminateIsIdempotent(t *testing.T) {
	t.Parallel()

	m := newRealManager()

	var calls []string
	m.Register(&recordingProvider{name: "p", scoped: true, calls: &calls})

	scope, err := m.Scope(context.Background())
	require.NoError(t, err)

	require.NoError(t, scope.Terminate(context.Background()))
	require.NoError(t, scope.Terminate(context.Background()))

	terminations := 0
	for _, c := range calls {
		if c == "p.Terminate" {
			terminations++
		}
	}
	assert.Equal(t, 1, terminations)
}

func TestScope_TerminateReturnsSameError(t *testing.T) {
	t.Parallel()

	m := newRealManager()
	termErr := errors.New("terminate failed")

	m.Register(&recordingProvider{name: "p", scoped: true, termErr: termErr})

	scope, err := m.Scope(context.Background())
	require.NoError(t, err)

	require.ErrorIs(t, scope.Terminate(context.Background()), termErr)
	require.ErrorIs(t, scope.Terminate(context.Background()), termErr)
}

func TestScope_AutoTerminationOnContextCancel(t *testing.T) {
	t.Parallel()

	m := newRealManager()

	terminated := make(chan struct{})
	m.Register(&recordingProvider{
		name:   "p",
		scoped: true,
		onTerminate: func(context.Context) error {
			close(terminated)
			return nil
		},
	})

	ctx, cancel := context.WithCancel(context.Background())
	_, err := m.Scope(ctx, WithAutoTermination())
	require.NoError(t, err)

	// The scope stays alive until the context is cancelled.
	select {
	case <-terminated:
		t.Fatal("scope terminated before context was cancelled")
	case <-time.After(20 * time.Millisecond):
	}

	cancel()

	select {
	case <-terminated:
	case <-time.After(time.Second):
		t.Fatal("scope was not auto-terminated after context cancellation")
	}
}

func TestScope_AutoTerminationDoesNotDoubleTerminate(t *testing.T) {
	t.Parallel()

	m := newRealManager()

	var count atomic.Int32
	m.Register(&recordingProvider{
		name:   "p",
		scoped: true,
		onTerminate: func(context.Context) error {
			count.Add(1)
			return nil
		},
	})

	ctx, cancel := context.WithCancel(context.Background())
	scope, err := m.Scope(ctx, WithAutoTermination())
	require.NoError(t, err)

	// Terminate manually, then cancel: the watcher must not terminate again.
	require.NoError(t, scope.Terminate(context.Background()))
	cancel()
	time.Sleep(20 * time.Millisecond)

	assert.Equal(t, int32(1), count.Load())
}

func TestRegister_ScopedProviderNotRunByGlobalRun(t *testing.T) {
	t.Parallel()

	m := newRealManager()

	var calls []string
	m.Register(&recordingProvider{name: "global", calls: &calls})
	m.Register(&recordingProvider{name: "scoped", scoped: true, calls: &calls})

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	require.NoError(t, m.Run(ctx, WithTerminationDelay(0)))

	assert.Contains(t, calls, "global.Register")
	assert.NotContains(t, calls, "scoped.Register")
	assert.NotContains(t, calls, "scoped.Boot")
}

func TestRegister_HasScopeFalseRunsGlobally(t *testing.T) {
	t.Parallel()

	m := newRealManager()

	var calls []string
	// Implements HasScope but opts out, so it must run at global boot, not per-scope.
	m.Register(&recordingProvider{name: "p", scoped: false, calls: &calls})

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	require.NoError(t, m.Run(ctx, WithTerminationDelay(0)))
	assert.Contains(t, calls, "p.Register")
	assert.Contains(t, calls, "p.Boot")

	// And nothing was registered as scoped.
	scope, err := m.Scope(context.Background())
	require.NoError(t, err)
	require.NoError(t, scope.Terminate(context.Background()))
	assert.Equal(t, []string{"p.Register", "p.Boot", "p.Terminate"}, calls)
}

func TestScope_NoScopedProviders(t *testing.T) {
	t.Parallel()

	m := newRealManager()

	scope, err := m.Scope(context.Background(), WithValue("k", "v"))
	require.NoError(t, err)
	require.NotNil(t, scope)
	require.NoError(t, scope.Terminate(context.Background()))
}

// TestScope_ConcurrentRegisterAndScope exercises the snapshot taken under lock:
// scopes are created while Register mutates the scoped provider map. Run with
// -race to catch any data race.
func TestScope_ConcurrentRegisterAndScope(t *testing.T) {
	t.Parallel()

	m := newRealManager()

	var wg sync.WaitGroup
	for i := 0; i < 50; i++ {
		wg.Add(2)
		go func() {
			defer wg.Done()
			m.Register(&recordingProvider{name: "p", scoped: true})
		}()
		go func() {
			defer wg.Done()
			scope, err := m.Scope(context.Background())
			assert.NoError(t, err)
			assert.NoError(t, scope.Terminate(context.Background()))
		}()
	}
	wg.Wait()
}
