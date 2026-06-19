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
- Global instance for small applications
- Concurrency-safe with no race conditions
- Works with any `Container` implementation (e.g. [danceable/container](https://github.com/danceable/container))

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
    }, bind.Singleton())
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
`provider.Default` — for convenience in small applications. Instead of creating
a manager with `provider.New()`, you can call `provider.Register()` and
`provider.Run()` directly as package-level functions; they all delegate to
`provider.Default`.

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

For more control, create your own `Manager` with a specific container.

```go
c := container.New()
m := provider.New(c)

m.Register(&DatabaseProvider{})
m.Register(&CacheProvider{})

ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt)
defer cancel()

if err := m.Run(ctx); err != nil {
    log.Fatal(err)
}
```

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
if err := scope.Container().Resolve(&user, resolve.WithName("user")); err != nil {
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

#### Manager Methods

| Method | Signature | Description |
|--------|-----------|-------------|
| New | `New(container Container) *Manager` | Creates a new manager instance with the given container. |
| Register | `Register(provider Provider)` | Registers a service provider. Providers implementing `HasScope` and returning `true` are stored as scoped providers; all others run at global boot. |
| Run | `Run(ctx context.Context, opts ...Option) error` | Runs the full lifecycle: register → boot → wait for context cancellation → terminate. |
| Scope | `Scope(ctx, opts ...ScopeOption) (*Scope, error)` | Opens a scoped instance (ephemeral by default; see options) and returns a handle the caller must `Terminate` unless `WithAutoTermination` is set. |

#### Scope Options

| Option | Description |
|--------|-------------|
| `WithValue(name string, value any)` | Seeds the scoped container with `value`, bound as a named singleton and resolvable via `resolve.WithName(name)`. A nil value returns `ErrNilScopeValue`. |
| `WithPersistent(name string)` | Makes the scope a named, persistent child (`container.Scope`) instead of the default ephemeral one (`container.Derive`). |
| `WithAutoTermination()` | Terminates the scope automatically once the context passed to `Scope` is cancelled. Teardown runs exactly once. |

#### Interfaces

| Interface | Methods | Description |
|-----------|---------|-------------|
| Container | `Reset()`, `Bind(...)`, `Call(...)`, `Resolve(...)`, `Fill(...)`, `Scope(name)`, `Derive()` | Dependency injection container used by providers to register and resolve bindings. |
| Provider | `Register(ctx, container)`, `Boot(ctx, container)`, `Terminate(ctx)` | Service provider that participates in the managed lifecycle. |
| HasOrder | `Order() int` | Optional interface for providers to specify execution priority. Lower values execute first. |
| HasScope | `Scoped() bool` | Optional interface for providers to opt into scoped execution. Returning `true` makes `Register` store the provider as scoped. |

The handle returned by `Scope` is a concrete `*Scope`:

| Type | Methods | Description |
|------|---------|-------------|
| Scope | `Name() string`, `Container() Container`, `Terminate(ctx) error` | A live scoped instance: the child container plus its booted scoped providers. |