// Command blog is a small DDD-structured blog that demonstrates wiring an
// application together with github.com/danceable/provider.
//
// Layers:
//
//	domain/         entities and ports (no external dependencies)
//	application/    use cases orchestrating the domain
//	infrastructure/ adapters (MongoDB, in-memory), HTTP server assembly, and the service providers
//	presenation/    HTTP handlers
//
// The providers register their bindings into the DI container, the manager
// boots them in order, and a SIGINT/SIGTERM triggers a graceful shutdown.
package main

import (
	"context"
	"log"
	"os/signal"
	"syscall"
	"time"

	"github.com/danceable/provider"
	blogprovider "github.com/danceable/provider/examples/blog/infrastructure/provider"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	provider.Register(blogprovider.NewConfigProvider())
	provider.Register(blogprovider.NewI18nProvider())
	provider.Register(blogprovider.NewTranslatorProvider())
	provider.Register(blogprovider.NewMongoProvider())
	provider.Register(blogprovider.NewArticleProvider())
	provider.Register(blogprovider.NewHTTPProvider())

	err := provider.Run(ctx,
		provider.WithTerminationDelay(100*time.Millisecond),
		provider.WithTerminationDeadline(15*time.Second),
	)
	if err != nil {
		log.Fatalf("blog: %v", err)
	}
}
