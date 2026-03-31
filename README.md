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

#### Manager Methods

| Method | Signature | Description |
|--------|-----------|-------------|
| New | `New(container Container) *Manager` | Creates a new manager instance with the given container. |
| Register | `Register(provider Provider)` | Registers a service provider with the manager. |
| Run | `Run(ctx context.Context, opts ...Option) error` | Runs the full lifecycle: register → boot → wait for context cancellation → terminate. |

#### Interfaces

| Interface | Methods | Description |
|-----------|---------|-------------|
| Container | `Reset()`, `Bind(...)`, `Call(...)`, `Resolve(...)`, `Fill(...)` | Dependency injection container used by providers to register and resolve bindings. |
| Provider | `Register(ctx, container)`, `Boot(ctx, container)`, `Terminate(ctx)` | Service provider that participates in the managed lifecycle. |
| HasOrder | `Order() int` | Optional interface for providers to specify execution priority. Lower values execute first. |