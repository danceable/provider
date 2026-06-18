// Package config loads the blog configuration from the environment.
//
// It lives in the infrastructure layer because it adapts an external concern
// (the process environment) into plain values that the service providers read
// while wiring the dependency graph.
package config

import (
	"os"
	"strconv"
)

// Config holds every value the blog needs to boot.
type Config struct {
	// HTTPAddr is the address the HTTP server listens on, e.g. ":8080".
	HTTPAddr string

	// MongoURI is the MongoDB connection string.
	MongoURI string

	// MongoDB is the name of the database that stores the articles.
	MongoDB string

	// PerPage is the number of articles shown per page on the public listing.
	PerPage int
}

// FromEnv builds a Config from environment variables, falling back to sensible
// defaults so the example runs out of the box with `go run .`.
func FromEnv() (*Config, error) {
	perPage, err := strconv.Atoi(env("BLOG_PER_PAGE", "5"))
	if err != nil {
		return nil, err
	}

	return &Config{
		HTTPAddr: env("BLOG_HTTP_ADDR", ":8080"),
		MongoURI: env("BLOG_MONGO_URI", "mongodb://localhost:27017"),
		MongoDB:  env("BLOG_MONGO_DB", "blog"),
		PerPage:  perPage,
	}, nil
}

// env returns the value of the environment variable, or fallback when unset/empty.
func env(key, fallback string) string {
	if value, ok := os.LookupEnv(key); ok && value != "" {
		return value
	}

	return fallback
}
