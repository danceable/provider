package provider

import (
	"context"
	"errors"
	"log"
	"net/http"
	"time"

	"github.com/danceable/container/bind"
	"github.com/danceable/provider"
	app "github.com/danceable/provider/examples/blog/application/article"
	"github.com/danceable/provider/examples/blog/infrastructure/config"
	"github.com/danceable/provider/examples/blog/infrastructure/i18n"
	"github.com/danceable/provider/examples/blog/infrastructure/render"
	"github.com/danceable/provider/examples/blog/presenation/http/handlers"
	"github.com/danceable/provider/examples/blog/presenation/http/middlewares"
)

// HTTPProvider builds the HTTP server from the article service and runs it. It
// starts listening on Boot and shuts down gracefully on Terminate.
type HTTPProvider struct {
	server *http.Server
}

var _ provider.Provider = (*HTTPProvider)(nil)

// NewHTTPProvider creates an HTTPProvider.
func NewHTTPProvider() *HTTPProvider { return &HTTPProvider{} }

// Order makes the HTTP server the last thing to boot and the first to terminate.
func (p *HTTPProvider) Order() int { return 30 }

// Register binds *http.Server, assembling the HTTP handler from the resolved
// application service, configuration and internationalization dependencies.
func (p *HTTPProvider) Register(_ context.Context, c provider.Container) error {
	return c.Bind(func(svc *app.Service, cfg *config.Config, repo i18n.Repository, scoper middlewares.Scoper) (*http.Server, error) {
		handler, err := NewServer(svc, cfg.PerPage, scoper, i18n.NewTranslator(repo, i18n.Default))
		if err != nil {
			return nil, err
		}

		return &http.Server{
			Addr:              cfg.HTTPAddr,
			Handler:           handler,
			ReadHeaderTimeout: 10 * time.Second,
		}, nil
	}, bind.Singleton(), bind.Lazy())
}

// Boot resolves the server and serves in the background so the manager can
// continue and block on the shutdown signal.
func (p *HTTPProvider) Boot(_ context.Context, c provider.Container) error {
	var server *http.Server
	if err := c.Resolve(&server); err != nil {
		return err
	}
	p.server = server

	go func() {
		log.Printf("blog: listening on %s", server.Addr)
		if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Printf("blog: http server error: %v", err)
		}
	}()

	return nil
}

// Terminate gracefully shuts the server down within the provided context's deadline.
func (p *HTTPProvider) Terminate(ctx context.Context) error {
	if p.server == nil {
		return nil
	}

	return p.server.Shutdown(ctx)
}

// NewServer assembles the blog's HTTP handler: it builds the public and
// dashboard handlers from the application service over a shared renderer,
// registers the routes, wraps the mux with the per-request i18n middleware (so
// every page renders in the visitor's language) and the request logger.
//
// defaultT is the renderer's fallback translator, used only when a request
// carries none; scoper lets the i18n middleware open a request scope per call.
func NewServer(svc *app.Service, perPage int, scoper middlewares.Scoper, defaultT *i18n.Translator) (http.Handler, error) {
	renderer, err := render.New(defaultT)
	if err != nil {
		return nil, err
	}

	public := handlers.NewPublic(svc, renderer, perPage)
	dashboard := handlers.NewDashboard(svc, renderer, perPage)

	mux := http.NewServeMux()

	// Public pages.
	mux.HandleFunc("GET /{$}", public.Home)
	mux.HandleFunc("GET /articles/{id}", public.Show)

	// Admin dashboard (CRUD). HTML forms only speak GET/POST, so writes are
	// modelled as POSTs to dedicated routes.
	mux.HandleFunc("GET /dashboard", dashboard.Dashboard)
	mux.HandleFunc("GET /dashboard/articles/new", dashboard.NewForm)
	mux.HandleFunc("POST /dashboard/articles", dashboard.Create)
	mux.HandleFunc("GET /dashboard/articles/{id}/edit", dashboard.EditForm)
	mux.HandleFunc("POST /dashboard/articles/{id}", dashboard.Update)
	mux.HandleFunc("POST /dashboard/articles/{id}/delete", dashboard.Delete)

	return logging(middlewares.WithI18n(scoper, mux)), nil
}

// logging is a tiny request logger so the example shows traffic on stdout.
func logging(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		next.ServeHTTP(w, r)
		log.Printf("%s %s %s", r.Method, r.URL.Path, time.Since(start))
	})
}
