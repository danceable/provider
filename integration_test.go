package provider_test

import (
	"context"
	"reflect"
	"sync"
	"testing"

	"github.com/danceable/container"
	"github.com/danceable/provider"
	"github.com/danceable/provider/adapters/danceable"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// These tests wire the package the way an application does: a real container
// behind its adapter, providers that bind and resolve real values, and
// assertions on what came out the other end — the sequence of phases, and the
// objects the container hands back.

// The services the providers below wire together.

type database struct{ dsn string }

type repository struct{ db *database }

type api struct{ repo *repository }

type cache struct{}

type disk struct{}

type gateway interface{ vendor() string }

type stripe struct{}

func (stripe) vendor() string { return "stripe" }

type paypal struct{}

func (paypal) vendor() string { return "paypal" }

// journal records the phases the providers go through, in order.
type journal struct {
	mu      sync.Mutex
	entries []string
}

func (j *journal) record(entry string) {
	j.mu.Lock()
	defer j.mu.Unlock()

	j.entries = append(j.entries, entry)
}

func (j *journal) snapshot() []string {
	j.mu.Lock()
	defer j.mu.Unlock()

	return append([]string(nil), j.entries...)
}

// service is the provider used throughout these tests. It implements every
// optional interface, so a scenario picks its behaviour by field: an order, the
// scoped flag, the types it provides (empty means it runs at boot), and hooks
// for the work each phase does.
type service struct {
	name     string
	order    int
	scoped   bool
	provides []reflect.Type

	onRegister func(context.Context, provider.Container) error
	onBoot     func(context.Context, provider.Container) error

	journal *journal
}

func (s *service) Order() int               { return s.order }
func (s *service) Scoped() bool             { return s.scoped }
func (s *service) Provides() []reflect.Type { return s.provides }

func (s *service) Register(ctx context.Context, c provider.Container) error {
	s.journal.record(s.name + ".Register")

	if s.onRegister != nil {
		return s.onRegister(ctx, c)
	}

	return nil
}

func (s *service) Boot(ctx context.Context, c provider.Container) error {
	s.journal.record(s.name + ".Boot")

	if s.onBoot != nil {
		return s.onBoot(ctx, c)
	}

	return nil
}

func (s *service) Terminate(context.Context) error {
	s.journal.record(s.name + ".Terminate")

	return nil
}

// boot runs the manager to completion. When during is given it is called with
// the booted container before shutdown; it must not assert, so that failures are
// reported from the test's own goroutine.
func boot(t *testing.T, m *provider.Manager, during func(provider.Container)) error {
	t.Helper()

	ctx, cancel := context.WithCancel(context.Background())

	if during == nil {
		cancel()

		return m.Run(ctx, provider.WithTerminationDelay(0))
	}

	done := make(chan struct{})
	err := m.Run(ctx, provider.WithTerminationDelay(0), provider.WithCallback(
		func(_ context.Context, c provider.Container) {
			defer close(done)
			defer cancel()

			during(c)
		},
	))
	if err != nil {
		return err
	}
	<-done

	return nil
}

func newManager() *provider.Manager { return provider.New(danceable.New(container.New())) }

// 1. Three eager providers, each registering against what the previous one
// bound: the order decides whether the chain wires up at all.
func TestIntegration_EagerProvidersRegisterInOrder(t *testing.T) {
	t.Parallel()

	build := func(j *journal, dbOrder, repoOrder, apiOrder int) *provider.Manager {
		m := newManager()

		// A binds the database.
		m.Register(&service{
			name: "A", order: dbOrder, journal: j,
			onRegister: func(_ context.Context, c provider.Container) error {
				return c.Bind(func() *database { return &database{dsn: "root@/app"} }, provider.Singleton())
			},
		})

		// B needs A's database while it registers.
		m.Register(&service{
			name: "B", order: repoOrder, journal: j,
			onRegister: func(_ context.Context, c provider.Container) error {
				var db *database
				if err := c.Resolve(&db); err != nil {
					return err
				}

				return c.Bind(func() *repository { return &repository{db: db} }, provider.Singleton())
			},
		})

		// C needs B's repository while it registers.
		m.Register(&service{
			name: "C", order: apiOrder, journal: j,
			onRegister: func(_ context.Context, c provider.Container) error {
				var repo *repository
				if err := c.Resolve(&repo); err != nil {
					return err
				}

				return c.Bind(func() *api { return &api{repo: repo} }, provider.Singleton())
			},
		})

		return m
	}

	t.Run("dependencies ordered first", func(t *testing.T) {
		t.Parallel()

		j := &journal{}
		m := build(j, 1, 2, 3)

		var built *api
		require.NoError(t, boot(t, m, func(c provider.Container) { _ = c.Resolve(&built) }))

		// Registration follows the order, booting follows it again, and
		// termination unwinds it.
		assert.Equal(t, []string{
			"A.Register", "B.Register", "C.Register",
			"A.Boot", "B.Boot", "C.Boot",
			"C.Terminate", "B.Terminate", "A.Terminate",
		}, j.snapshot())

		// And the chain is wired all the way down.
		require.NotNil(t, built)
		assert.Equal(t, "root@/app", built.repo.db.dsn)
	})

	t.Run("dependency ordered last", func(t *testing.T) {
		t.Parallel()

		j := &journal{}
		m := build(j, 3, 1, 2) // the database now registers last

		err := boot(t, m, nil)

		// B registers first and finds nothing to build on, so startup fails
		// before anything boots.
		require.ErrorContains(t, err, "no concrete found")
		assert.Equal(t, []string{"B.Register"}, j.snapshot())
	})
}

// 2. Two eager providers, each pulling a different deferred provider while it
// registers.
func TestIntegration_EagerProvidersPullDeferredOnesAtRegistration(t *testing.T) {
	t.Parallel()

	j := &journal{}
	m := newManager()

	// A (eager) needs what C provides.
	m.Register(&service{
		name: "A", order: 1, journal: j,
		onRegister: func(_ context.Context, c provider.Container) error {
			var dep *cache
			return c.Resolve(&dep)
		},
	})

	// B (eager) needs what D provides.
	m.Register(&service{
		name: "B", order: 2, journal: j,
		onRegister: func(_ context.Context, c provider.Container) error {
			var dep *disk
			return c.Resolve(&dep)
		},
	})

	// C and D are deferred, and ordered after the providers that use them so
	// they are terminated first.
	m.Register(&service{
		name: "C", order: 10, journal: j,
		provides: []reflect.Type{reflect.TypeFor[*cache]()},
		onRegister: func(_ context.Context, c provider.Container) error {
			return c.Bind(func() *cache { return &cache{} }, provider.Singleton())
		},
	})
	m.Register(&service{
		name: "D", order: 20, journal: j,
		provides: []reflect.Type{reflect.TypeFor[*disk]()},
		onRegister: func(_ context.Context, c provider.Container) error {
			return c.Bind(func() *disk { return &disk{} }, provider.Singleton())
		},
	})

	require.NoError(t, boot(t, m, nil))

	// Each deferred provider runs both its phases at the point it is requested,
	// nested inside the registration that asked for it.
	assert.Equal(t, []string{
		"A.Register", "C.Register", "C.Boot",
		"B.Register", "D.Register", "D.Boot",
		"A.Boot", "B.Boot",
		"D.Terminate", "C.Terminate", "B.Terminate", "A.Terminate",
	}, j.snapshot())
}

// 3. An eager provider pulls a deferred one, which pulls another deferred one:
// the load nests depth-first.
func TestIntegration_DeferredProviderPullsAnotherAtRegistration(t *testing.T) {
	t.Parallel()

	j := &journal{}
	m := newManager()

	var built *repository

	// A (eager) needs the repository C provides.
	m.Register(&service{
		name: "A", order: 1, journal: j,
		onRegister: func(_ context.Context, c provider.Container) error {
			return c.Resolve(&built)
		},
	})

	// B (eager) is independent.
	m.Register(&service{name: "B", order: 2, journal: j})

	// C is deferred and needs the database D provides, while it registers.
	m.Register(&service{
		name: "C", order: 10, journal: j,
		provides: []reflect.Type{reflect.TypeFor[*repository]()},
		onRegister: func(_ context.Context, c provider.Container) error {
			var db *database
			if err := c.Resolve(&db); err != nil {
				return err
			}

			return c.Bind(func() *repository { return &repository{db: db} }, provider.Singleton())
		},
	})

	// D is deferred and binds the database.
	m.Register(&service{
		name: "D", order: 20, journal: j,
		provides: []reflect.Type{reflect.TypeFor[*database]()},
		onRegister: func(_ context.Context, c provider.Container) error {
			return c.Bind(func() *database { return &database{dsn: "root@/deep"} }, provider.Singleton())
		},
	})

	require.NoError(t, boot(t, m, nil))

	// D is completely up before C finishes registering, and C before A returns.
	assert.Equal(t, []string{
		"A.Register",
		"C.Register",
		"D.Register", "D.Boot",
		"C.Boot",
		"B.Register",
		"A.Boot", "B.Boot",
		"D.Terminate", "C.Terminate", "B.Terminate", "A.Terminate",
	}, j.snapshot())

	// A got a repository backed by the database the deepest provider bound.
	require.NotNil(t, built)
	assert.Equal(t, "root@/deep", built.db.dsn)
}

// 4. Two deferred providers that provide the same type. One request loads both,
// lowest order first, and the lowest order wins the singleton slot.
func TestIntegration_CompetingDeferredProvidersResolveByOrder(t *testing.T) {
	t.Parallel()

	j := &journal{}
	m := newManager()

	var chosen gateway

	// A needs a gateway while it registers.
	m.Register(&service{
		name: "A", order: 1, journal: j,
		onRegister: func(_ context.Context, c provider.Container) error {
			return c.Resolve(&chosen)
		},
	})

	// B and C both provide it; C is ordered lower.
	m.Register(&service{
		name: "B", order: 20, journal: j,
		provides: []reflect.Type{reflect.TypeFor[gateway]()},
		onRegister: func(_ context.Context, c provider.Container) error {
			return c.Bind(func() gateway { return paypal{} }, provider.Singleton())
		},
	})
	m.Register(&service{
		name: "C", order: 10, journal: j,
		provides: []reflect.Type{reflect.TypeFor[gateway]()},
		onRegister: func(_ context.Context, c provider.Container) error {
			return c.Bind(func() gateway { return stripe{} }, provider.Singleton())
		},
	})

	require.NoError(t, boot(t, m, nil))

	// Both providers load — the trigger set is what decides, not the binding —
	// and they load lowest order first.
	assert.Equal(t, []string{
		"A.Register",
		"C.Register", "C.Boot",
		"B.Register", "B.Boot",
		"A.Boot",
		"B.Terminate", "C.Terminate", "A.Terminate",
	}, j.snapshot())

	// The singleton slot is filled once, so the lower order won it.
	require.NotNil(t, chosen)
	assert.Equal(t, "stripe", chosen.vendor())
}

// 5. A scoped provider registering against a binding that lives on an ancestor
// container.
func TestIntegration_ScopedProviderResolvesFromAnAncestor(t *testing.T) {
	t.Parallel()

	j := &journal{}
	m := newManager()

	// A (eager) binds the database on the manager's container.
	m.Register(&service{
		name: "A", order: 1, journal: j,
		onRegister: func(_ context.Context, c provider.Container) error {
			return c.Bind(func() *database { return &database{dsn: "root@/app"} }, provider.Singleton())
		},
	})

	// B (eager) is independent.
	m.Register(&service{name: "B", order: 2, journal: j})

	// C is scoped: it reaches up to A's database and binds a repository that
	// only its own scope can see.
	m.Register(&service{
		name: "C", order: 3, scoped: true, journal: j,
		onRegister: func(_ context.Context, c provider.Container) error {
			var db *database
			if err := c.Resolve(&db); err != nil { // upward traversal to the root
				return err
			}

			return c.Bind(func() *repository { return &repository{db: db} }, provider.Singleton())
		},
	})

	var (
		rootDB                *database
		firstRepo, secondRepo *repository
		rootRepoErr           error
		firstID               string
		scopeErr              error
	)

	require.NoError(t, boot(t, m, func(c provider.Container) {
		_ = c.Resolve(&rootDB)

		first, err := m.Scope(context.Background(), provider.WithValue("requestID", "req-1"))
		if err != nil {
			scopeErr = err

			return
		}

		_ = first.Container().Resolve(&firstRepo)
		_ = first.Container().Resolve(&firstID, provider.ResolveName("requestID"))

		second, err := m.Scope(context.Background())
		if err != nil {
			scopeErr = err

			return
		}

		_ = second.Container().Resolve(&secondRepo)

		// The scoped binding stays in its scope.
		var leaked *repository
		rootRepoErr = c.Resolve(&leaked)

		_ = first.Terminate(context.Background())
		_ = second.Terminate(context.Background())
	}))

	require.NoError(t, scopeErr)

	// The eager providers ran once at boot; the scoped one ran per scope and was
	// never run by Run itself.
	assert.Equal(t, []string{
		"A.Register", "B.Register", "A.Boot", "B.Boot",
		"C.Register", "C.Boot", // first scope
		"C.Register", "C.Boot", // second scope
		"C.Terminate", "C.Terminate",
		"B.Terminate", "A.Terminate",
	}, j.snapshot())

	// Each scope built its own repository, both backed by the one database the
	// eager provider bound on the ancestor container.
	require.NotNil(t, firstRepo)
	require.NotNil(t, secondRepo)
	assert.NotSame(t, firstRepo, secondRepo, "each scope registers its own")
	assert.Same(t, rootDB, firstRepo.db, "resolved upward from the scope")
	assert.Same(t, rootDB, secondRepo.db)

	// The seeded value is visible inside the scope, and the scoped binding is not
	// visible from the root.
	assert.Equal(t, "req-1", firstID)
	assert.ErrorContains(t, rootRepoErr, "no concrete found")
}
