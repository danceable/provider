# Blog example

A small blog built on top of [`github.com/danceable/provider`](../../) to show
how to wire an application with the service-provider container, following a
Domain-Driven Design layering and backed by MongoDB.

## Features

- Public pages
  - `GET /` — paginated list of articles (newest first)
  - `GET /articles/{id}` — article detail
- Admin dashboard at `/dashboard` — full CRUD over articles
  (`id`, `title`, `body`, `created_at`)
- Server-rendered HTML (`html/template`, embedded)
- MongoDB persistence + Docker Compose

> The dashboard is intentionally **unauthenticated** to keep the example
> focused on dependency injection and DDD. Add auth middleware in
> [`infrastructure/provider`](infrastructure/provider) before exposing it publicly.

## Layout

```
domain/article          Article entity, invariants, Repository port (no deps)
application/article     Use cases (Create/Update/Delete/Get/List) over the port
infrastructure/
  config                Environment configuration
  mongodb               MongoDB connection client
  memory                In-memory Repository adapter (used by tests / local runs)
  repositories/mongodb  MongoDB Repository adapter
  render                HTML template renderer
  provider              Service providers wiring the DI container, plus the
                        HTTP server assembly and router
presenation/http        HTTP handlers
resources/templates     Embedded HTML templates
main.go                 Registers the providers and runs the manager
```

### How the container is used

Each provider implements `provider.Provider` (`Register` / `Boot` / `Terminate`)
and declares an `Order()`:

| Order | Provider          | Binds                                   | Boot / Terminate                |
|-------|-------------------|-----------------------------------------|---------------------------------|
| 0     | `ConfigProvider`  | `*config.Config`                        | —                               |
| 10    | `MongoProvider`   | `*mongo.Client`, `*mongo.Database`      | pings on boot / disconnects     |
| 20    | `ArticleProvider` | `domain.Repository`, `*app.Service`     | —                               |
| 30    | `HTTPProvider`    | `*http.Server`                          | serves on boot / graceful close |

Dependencies are resolved by type through the container: the HTTP handler
depends on the application service, which depends on the `domain.Repository`
port, which is bound to the MongoDB adapter, which depends on the database and
config. Swapping MongoDB for the in-memory adapter is a one-line change in
`ArticleProvider`.

## Run with Docker (recommended)

```bash
docker compose up --build
```

Then open <http://localhost:8080> and the dashboard at
<http://localhost:8080/dashboard>.

## Run locally

Requires a MongoDB reachable at `BLOG_MONGO_URI` (defaults to
`mongodb://localhost:27017`).

```bash
go run .
```

Configuration is read from the environment (see [.env.example](.env.example)).

## Tests

```bash
go test ./...
```

The domain, application and HTTP handlers are tested against the in-memory
repository, so no MongoDB instance is needed to run them.
