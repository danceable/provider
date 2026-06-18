// Package mongodb is the MongoDB adapter for the article repository port and
// the place where the low-level driver is configured.
package mongodb

import (
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

// Connect opens a MongoDB client for the given connection URI. In the v2
// driver Connect does not perform any network I/O; the connection is verified
// later with Client.Ping (see the Mongo provider's Boot step).
func Connect(uri string) (*mongo.Client, error) {
	return mongo.Connect(options.Client().ApplyURI(uri))
}
