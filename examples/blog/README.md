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
- Multi-language UI (English, German, Persian, Chinese) driven by a **scoped
  provider** — see [Internationalization](#internationalization-scoped-providers)
- MongoDB persistence + Docker Compose

> The dashboard is intentionally **unauthenticated** to keep the example
> focused on dependency injection and DDD. Add auth middleware in
> [`infrastructure/provider`](infrastructure/provider) before exposing it publicly.

## Layout

```
domain/article          Article entity, invariants, Repository port (no deps)
application/article     Use cases (Create/Update/Delete/Get/List) over the port
infrastructure/
  i18n                  Language, translation Repository port, Translator
  config                Environment configuration
  mongodb               MongoDB connection client
  repositories/memory   In-memory adapters: article Repository + translations
  repositories/mongodb  MongoDB Repository adapter
  render                HTML template renderer (per-request translation funcs)
  provider              Service providers wiring the DI container, plus the
                        HTTP server assembly and router
presenation/http/
  handlers              HTTP handlers (public pages + dashboard CRUD)
  middlewares           HTTP middlewares (per-request i18n)
resources/templates     Embedded HTML templates (text is translation keys)
main.go                 Registers the providers and runs the manager
```

### How the container is used

Each provider implements `provider.Provider` (`Register` / `Boot` / `Terminate`)
and declares an `Order()`:

| Order | Provider          | Binds                                   | Boot / Terminate                |
|-------|-------------------|-----------------------------------------|---------------------------------|
| 0     | `ConfigProvider`  | `*config.Config`                        | —                               |
| 5     | `I18nProvider`    | `i18n.Repository`, `middlewares.Scoper` | —                               |
| 10    | `MongoProvider`   | `*mongo.Client`, `*mongo.Database`      | pings on boot / disconnects     |
| 20    | `ArticleProvider` | `domain.Repository`, `*app.Service`     | —                               |
| 30    | `HTTPProvider`    | `*http.Server`                          | serves on boot / graceful close |

Dependencies are resolved by type through the container: the HTTP handler
depends on the application service, which depends on the `domain.Repository`
port, which is bound to the MongoDB adapter, which depends on the database and
config. Swapping MongoDB for the in-memory adapter is a one-line change in
`ArticleProvider`.

In addition, `TranslatorProvider` is registered but implements
`provider.HasScope` (`Scoped() bool` → `true`), so the manager keeps it out of
global boot and instead runs it per request scope (see below).

### Internationalization (scoped providers)

The UI text lives entirely in the translation `Repository` as `key → value`
pairs; templates reference only keys through the `{{ t "key" }}` function. A
visitor's language is resolved per request and applied through a **scoped
provider**:

1. `WithI18n` middleware ([`presenation/http/middlewares/i18n.go`](presenation/http/middlewares/i18n.go))
   detects the language (`?lang=`, then the `lang` cookie, then
   `Accept-Language`, then the default) and opens a request scope seeded with it:
   `scoper.Scope(ctx, provider.WithValue(i18n.LanguageValue, lang))`.
2. The scoped `TranslatorProvider` runs against that child container and binds a
   `*i18n.Translator` for the seeded language (resolving the shared
   `i18n.Repository` from an ancestor scope).
3. The middleware resolves the `Translator`, stores it in the request context,
   and terminates the scope when the request returns. The renderer reads it to
   bind the per-request `t` / `lang` / `dir` template functions.

Languages: English (`en`), German (`de`), Persian/Farsi (`fa`, right-to-left)
and Chinese (`zh`). Switch with the header dropdown or `?lang=fa`. Missing keys fall
back to the default language and finally to the key itself.

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
