[![Go Reference](https://pkg.go.dev/badge/github.com/danceable/provider.svg)](https://pkg.go.dev/github.com/danceable/provider)
[![CI](https://github.com/danceable/provider/actions/workflows/ci.yml/badge.svg)](https://github.com/danceable/provider/actions/workflows/ci.yml)
[![CodeQL](https://github.com/danceable/provider/actions/workflows/codeql-analysis.yml/badge.svg)](https://github.com/danceable/provider/actions/workflows/codeql-analysis.yml)
[![Go Report Card](https://goreportcard.com/badge/github.com/danceable/provider)](https://goreportcard.com/report/github.com/danceable/provider)
[![Coverage Status](https://coveralls.io/repos/github/danceable/provider/badge.svg)](https://coveralls.io/github/danceable/provider?branch=main)

<p align="center">
  <img src="logo.svg" alt="Provider Logo" />
</p>

# Provider

Provider is a lightweight service provider manager for Go projects.
It manages the full lifecycle of service providers — registration, booting, and
graceful termination — on top of a dependency injection container.

Features:

- Three-phase lifecycle: Register → Boot → Terminate
- Ordered execution via the optional `HasOrder` interface
- Reverse-order termination for clean shutdown
- Scoped providers that run per request/job inside a child container seeded with `WithValue`
- Deferred providers that load the first time one of the types they provide is requested
- Global instance for small applications
- Concurrency-safe with no race conditions
- Backend-agnostic: works with any container through an adapter, with [danceable/container](https://github.com/danceable/container) shipped out of the box

## Documentation

### Required Go Versions

It requires Go `v1.26` or newer versions.

### Installation

To install this package, run the following command in your project directory.

```
go get github.com/danceable/provider
```

Next, include it in your application:

```go
import "github.com/danceable/provider"
```

### Introduction

Provider works by managing the lifecycle of service providers in three ordered
phases:

1. **Register** — Each provider registers its bindings into the container.
   Providers are called from lowest order to highest.
2. **Boot** — After all providers are registered, each provider is booted
   (again lowest to highest). This is the place for initialization logic that
   depends on bindings from other providers.
3. **Terminate** — When the context is cancelled the manager terminates every
   provider in **reverse** order (highest to lowest), allowing graceful cleanup.
   Termination covers every provider whose `Register` ran, including one whose
   `Boot` failed — `Register` alone may already have acquired something — so
   `Terminate` should release only what the provider actually holds.

A provider is any type that implements the `Provider` interface:

```go
type Provider interface {
    Register(ctx context.Context, container Container) error
    Boot(ctx context.Context, container Container) error
    Terminate(ctx context.Context) error
}
```

Optionally, a provider can implement `HasOrder` to control its execution
priority:

```go
type HasOrder interface {
    Order() int
}
```

Providers with lower order values execute first during Register and Boot, and
last during Terminate.

### Quick Start

The following example demonstrates registering and running a single provider.

```go
provider.Register(&MyProvider{})

ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt)
defer cancel()

if err := provider.Run(ctx); err != nil {
    log.Fatal(err)
}
```

### Examples

#### Implementing a Provider

```go
type DatabaseProvider struct{}

func (p *DatabaseProvider) Register(ctx context.Context, c provider.Container) error {
    return c.Bind(func() Database {
        return &MySQL{Host: "localhost", Port: 3306}
    }, provider.Singleton())
}

func (p *DatabaseProvider) Boot(ctx context.Context, c provider.Container) error {
    var db Database
    if err := c.Resolve(&db); err != nil {
        return err
    }
    return db.Connect()
}

func (p *DatabaseProvider) Terminate(ctx context.Context) error {
    // cleanup resources
    return nil
}
```

#### Global Instance

The package provides a default global `Manager` instance — exposed as
`provider.Default` — for convenience in small applications. It is backed by the
[danceable/container](https://github.com/danceable/container) global container
through the danceable adapter. Instead of creating a manager with
`provider.New()`, you can call `provider.Register()` and `provider.Run()`
directly as package-level functions; they all delegate to `provider.Default`.

```go
provider.Register(&DatabaseProvider{})
provider.Register(&CacheProvider{})

ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt)
defer cancel()

if err := provider.Run(ctx); err != nil {
    log.Fatal(err)
}
```

#### Custom Manager Instance

For more control, create your own `Manager` with a specific container. `New`
takes a `provider.Container`, which you obtain by wrapping a concrete container
in one of the adapters (see [Container Backends](#container-backends)).

```go
c := container.New()
m := provider.New(danceable.New(c))

m.Register(&DatabaseProvider{})
m.Register(&CacheProvider{})

ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt)
defer cancel()

if err := m.Run(ctx); err != nil {
    log.Fatal(err)
}
```

#### Container Backends

Provider is decoupled from any concrete dependency-injection container. Providers
bind and resolve through the neutral `provider.Container` interface using neutral
options (`provider.Singleton()`, `provider.Lazy()`, `provider.WithName()`,
`provider.ResolveName()`), and an **adapter** maps those onto a specific backend.
One adapter ships with the package:

| Backend | Adapter package | Construct with |
|---------|-----------------|----------------|
| [danceable/container](https://github.com/danceable/container) | `github.com/danceable/provider/adapters/danceable` | `danceable.New(container.New())` |

```go
import (
    "github.com/danceable/container"
    "github.com/danceable/provider"
    "github.com/danceable/provider/adapters/danceable"
)

m := provider.New(danceable.New(container.New()))
```

> To run the same providers on another container, implement `provider.Container`
> in an adapter of your own. Backends differ in capability, and the neutral
> options are the place that shows: an adapter whose bindings are always lazy and
> memoized has nothing to do for `provider.Singleton()` and `provider.Lazy()`, and
> one without runtime resolve parameters ignores `provider.WithParams()`.
>
> `provider.Default` is already wired to `container.Default` through the
> danceable adapter, so the global instance needs no wiring of its own.

#### Ordered Providers

Implement `HasOrder` to control the execution order. Providers with lower order
values are registered and booted first, and terminated last.

```go
type DatabaseProvider struct{}

func (p *DatabaseProvider) Order() int { return 1 }

func (p *DatabaseProvider) Register(ctx context.Context, c provider.Container) error { /* ... */ }
func (p *DatabaseProvider) Boot(ctx context.Context, c provider.Container) error     { /* ... */ }
func (p *DatabaseProvider) Terminate(ctx context.Context) error                      { /* ... */ }

type CacheProvider struct{}

func (p *CacheProvider) Order() int { return 2 }

func (p *CacheProvider) Register(ctx context.Context, c provider.Container) error { /* ... */ }
func (p *CacheProvider) Boot(ctx context.Context, c provider.Container) error     { /* ... */ }
func (p *CacheProvider) Terminate(ctx context.Context) error                      { /* ... */ }
```

With the above, the execution order is:

1. `DatabaseProvider.Register` → `CacheProvider.Register`
2. `DatabaseProvider.Boot` → `CacheProvider.Boot`
3. `CacheProvider.Terminate` → `DatabaseProvider.Terminate`

#### Lifecycle

`Run` blocks until the provided context is cancelled. A typical pattern uses
`signal.NotifyContext` so the application shuts down on an OS signal:

```go
ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
defer cancel()

if err := provider.Run(ctx); err != nil {
    log.Fatal(err)
}
```

#### Run Options

`Run` accepts functional options to customize termination behavior and register
a post-boot callback.

| Option | Description |
|--------|-------------|
| `WithTerminationDelay(d time.Duration)` | Duration to wait before starting termination after the context is cancelled. Default: `300ms`. |
| `WithTerminationDeadline(d time.Duration)` | Maximum duration allowed for all providers to terminate. Default: `200ms`. |
| `WithCallback(fn func(ctx context.Context, container Container))` | Function called (in a goroutine) after all providers have booted but before waiting for the context to be cancelled. Receives both the context and the container. |

```go
if err := provider.Run(ctx,
    provider.WithTerminationDelay(1*time.Second),
    provider.WithTerminationDeadline(10*time.Second),
    provider.WithCallback(func(ctx context.Context, c provider.Container) {
        log.Println("all providers booted")
    }),
); err != nil {
    log.Fatal(err)
}
```

#### Scoped Providers

Some providers should not live for the whole application — they belong to a
single request, job, or transaction. Mark these by implementing the optional
`HasScope` interface (`Scoped() bool`) and returning `true`; `Register` then
routes them to the scoped set automatically. Scoped providers are skipped by
`Run` and instead executed each time you open a scope, against a **child
container** derived from the manager's container.

```go
type RequestContextProvider struct{}

func (p *RequestContextProvider) Scoped() bool { return true }

func (p *RequestContextProvider) Register(ctx context.Context, c provider.Container) error { /* ... */ }
func (p *RequestContextProvider) Boot(ctx context.Context, c provider.Container) error     { /* ... */ }
func (p *RequestContextProvider) Terminate(ctx context.Context) error                      { /* ... */ }
```

Open a scope with `Scope(ctx, opts...)`. By default the scope is **anonymous and
ephemeral** — backed by `container.Derive`, it is garbage-collected once you drop
the returned `*Scope`, which is ideal per request/job. `WithValue(name, value)`
options seed the child before the scoped providers' `Register` then `Boot` run.
The returned `*Scope` exposes the child via `Container()`; you own its lifetime
and must call `Terminate` when the scope ends (scoped providers terminate in
reverse order, just like `Run`).

```go
m.Register(&RequestContextProvider{}) // routed to the scoped set via Scoped()

// Open a scope for one HTTP request, seeding request-specific values.
scope, err := m.Scope(r.Context(),
    provider.WithValue("requestID", reqID),
    provider.WithValue("user", currentUser),
)
if err != nil {
    return err
}
defer scope.Terminate(r.Context())

// Resolve a seeded value (bound as a named singleton).
var user *User
if err := scope.Container().Resolve(&user, provider.ResolveName("user")); err != nil {
    return err
}
```

Two options change the scope's lifetime:

- `WithPersistent(name)` makes the scope a **named, persistent** child
  (`container.Scope`) instead of an ephemeral one. The named child is cached on
  its parent and reused by later calls with the same name.
- `WithAutoTermination()` ties teardown to the context: the scope terminates
  itself once `ctx` is cancelled, so you don't have to call `Terminate`.
  Termination still runs exactly once, so combining it with an explicit
  `Terminate` is safe.

```go
// A persistent scope that cleans itself up when ctx is cancelled.
scope, err := m.Scope(ctx,
    provider.WithPersistent("worker"),
    provider.WithAutoTermination(),
    provider.WithValue("jobID", jobID),
)
```

> `Scope` is a method on `*Manager`; reach the global instance via
> `provider.Default.Scope(...)`.

#### Deferred Providers

Some providers are expensive to boot and are not needed on every run — a search
client, a PDF renderer, a payment gateway. Mark these by implementing the
optional `Deferrable` interface and listing the types the provider binds:

```go
type Deferrable interface {
    Provides() []reflect.Type
}
```

`Register` keeps such a provider with all the others; `Run` simply walks past
it. It is registered and booted the first time one of the types it provides is
requested from the container — through `Resolve`, `Call` or `Fill` — and never
at all if nothing asks for it.

```go
type SearchProvider struct{}

func (p *SearchProvider) Provides() []reflect.Type {
    return []reflect.Type{reflect.TypeFor[search.Client]()}
}

func (p *SearchProvider) Register(ctx context.Context, c provider.Container) error {
    return c.Bind(func() search.Client { return search.Dial(/* ... */) }, provider.Singleton())
}

func (p *SearchProvider) Boot(ctx context.Context, c provider.Container) error     { /* ... */ }
func (p *SearchProvider) Terminate(ctx context.Context) error                      { /* ... */ }
```

```go
m.Register(&SearchProvider{}) // registered like any other provider

if err := m.Run(ctx, provider.WithCallback(func(ctx context.Context, c provider.Container) {
    // Nothing has asked for a search.Client yet, so the provider has not booted.

    var client search.Client
    if err := c.Resolve(&client); err != nil { // registers and boots it now
        log.Fatal(err)
    }
})); err != nil {
    log.Fatal(err)
}
```

Details worth knowing:

- **A provided type also triggers on the interfaces it implements**, mirroring
  the container's own lookup fallback — binding `*redis.Client` and providing
  that type loads the provider when a `Cache` interface it implements is
  requested.
- **Loading is depth-first, and runs both phases.** The request that triggers a
  provider blocks until that provider is completely up — `Register` **and**
  `Boot` — and, when it pulls in further deferred providers, until those are up
  too. A chain `A → C → D` runs `D.Register`, `D.Boot`, then the rest of
  `C.Register`, then `C.Boot`, and only then does A's request return:

  ```
  A.Boot {
    C.Register {
      D.Register
      D.Boot
    C.Register }
    C.Boot
  A.Boot }
  ```

  A provider's dependencies are therefore fully booted before it binds anything.
- **A deferred provider may depend on another one, but it must ask for it.**
  Requesting a deferred type while a provider registers or boots loads the
  provider behind it too, so pulling a dependency through the container you were
  handed works:

  ```go
  func (p *RepositoryProvider) Register(ctx context.Context, c provider.Container) error {
      var db *sql.DB
      if err := c.Resolve(&db); err != nil { // loads the deferred database provider
          return err
      }

      return c.Bind(func(db *sql.DB) Repository { /* ... */ }, provider.Singleton(), provider.Lazy())
  }
  ```

  What does **not** work is leaving that dependency to the container: a resolver
  whose parameter is provided by a deferred provider that has not loaded fails
  with a missing binding. Only the types named at `Resolve`, `Call` and `Fill`
  trigger loading — the container resolves a resolver's own parameters
  internally, where nothing can intercept them. Pull the deferred types you
  depend on in `Register` or `Boot` and every later resolution finds them.

  A provider requesting one of its own types (the usual bind-in-`Register`,
  resolve-in-`Boot` pattern) is not affected: it never waits for itself. Two
  providers that need each other while loading are a cycle — the inner request
  falls through to the container and fails as a missing binding.
- **Loading happens once**, even when several goroutines request the type at the
  same time — the others wait for the load rather than racing past it. A load
  that fails is not retried; every request that triggers it reports the same
  error.
- **Two providers that provide the same type both load.** `Provides()` is the
  trigger set, so naming one type in two providers means a request loads both,
  lowest order first, and both are terminated later — the one whose binding is
  discarded still ran and still holds whatever it acquired. Which binding
  survives is the container's call, not this package's: a `provider.Singleton()` slot
  is filled once, so the **lowest** order wins, while a transient binding is
  overwritten, so the **highest** order wins. If you want a default and an
  override, separate them with `provider.WithName(...)` rather than relying on order.
- **A deferred provider keeps its place.** It sits in the same list as every
  other provider, at the order it declares, and terminates in that slot like the
  rest — deferral changes *when* it runs, not *where* it sits. Providers nothing
  ever needed are simply not terminated, since they never registered.
  `Order()` does not decide when a deferred provider **loads** — whoever asks for
  it first decides that — only where it sits in the teardown. Order a deferred
  provider above whatever depends on it, exactly as you would an eager one.
- **The context is the application's.** A deferred provider is registered and
  booted with the context passed to `Run`, even when a short-lived caller
  triggers the load, so its lifetime matches the application's.
- **`Provides()` returning nothing opts out**: nothing could ever trigger such a
  provider, so it takes part in the regular lifecycle instead.

Deferral combines with scoping: a provider that is both `Scoped()` and
`Deferrable` is deferred **within each scope**. Every scope gets its own copy,
loads it the first time the scope's container is asked for one of its types, and
terminates it with the scope — so a request that never touches the service pays
nothing for it.

```go
scope, err := m.Scope(r.Context())
if err != nil {
    return err
}
defer scope.Terminate(r.Context())

// Loads the scoped deferred provider that binds this type, for this scope only.
var reporting report.Builder
if err := scope.Container().Resolve(&reporting); err != nil {
    return err
}
```

#### Manager Methods

| Method | Signature | Description |
|--------|-----------|-------------|
| New | `New(container Container) *Manager` | Creates a new manager instance with the given container. |
| Register | `Register(provider Provider)` | Registers a service provider. Providers implementing `HasScope` and returning `true` are stored as scoped providers; all others run at global boot. Implementing `Deferrable` (with a non-empty `Provides`) makes a provider of either set run on demand instead of at boot. |
| Run | `Run(ctx context.Context, opts ...Option) error` | Runs the full lifecycle: register → boot → wait for context cancellation → terminate. |
| Scope | `Scope(ctx, opts ...ScopeOption) (*Scope, error)` | Opens a scoped instance (ephemeral by default; see options) and returns a handle the caller must `Terminate` unless `WithAutoTermination` is set. |

#### Scope Options

| Option | Description |
|--------|-------------|
| `WithValue(name string, value any)` | Seeds the scoped container with `value`, bound as a named singleton and resolvable via `provider.ResolveName(name)`. A nil value returns `ErrNilScopeValue`. |
| `WithPersistent(name string)` | Makes the scope a named, persistent child (`container.Scope`) instead of the default ephemeral one (`container.Derive`). |
| `WithAutoTermination()` | Terminates the scope automatically once the context passed to `Scope` is cancelled. Teardown runs exactly once. |

#### Interfaces

| Interface | Methods | Description |
|-----------|---------|-------------|
| Container | `Reset()`, `Bind(...)`, `Call(...)`, `Resolve(...)`, `Fill(...)`, `Scope(name)`, `Derive()` | Dependency injection container used by providers to register and resolve bindings. |
| Provider | `Register(ctx, container)`, `Boot(ctx, container)`, `Terminate(ctx)` | Service provider that participates in the managed lifecycle. |
| HasOrder | `Order() int` | Optional interface for providers to specify execution priority. Lower values execute first. |
| HasScope | `Scoped() bool` | Optional interface for providers to opt into scoped execution. Returning `true` makes `Register` store the provider as scoped. |
| Deferrable | `Provides() []reflect.Type` | Optional interface for providers to defer their registration until one of the returned types is requested from the container. An empty result opts out. |

The handle returned by `Scope` is a concrete `*Scope`:

| Type | Methods | Description |
|------|---------|-------------|
| Scope | `Name() string`, `Container() Container`, `Terminate(ctx) error` | A live scoped instance: the child container plus its booted scoped providers. |