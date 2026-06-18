package mongodb

import (
	"context"
	"errors"
	"time"

	domain "github.com/danceable/provider/examples/blog/domain/article"
	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

// collectionName is the MongoDB collection that stores the articles.
const collectionName = "articles"

// articleDoc is the persistence representation of an article. Keeping it
// separate from the domain entity stops MongoDB concerns (ObjectID, bson tags)
// from leaking into the domain.
type articleDoc struct {
	ID        bson.ObjectID `bson:"_id"`
	Title     string        `bson:"title"`
	Body      string        `bson:"body"`
	CreatedAt time.Time     `bson:"created_at"`
}

func (d articleDoc) toDomain() domain.Article {
	return domain.Article{
		ID:        d.ID.Hex(),
		Title:     d.Title,
		Body:      d.Body,
		CreatedAt: d.CreatedAt,
	}
}

// ArticleRepository is the MongoDB-backed domain.Repository.
type ArticleRepository struct {
	coll *mongo.Collection
}

// compile-time assertion that the adapter satisfies the domain port.
var _ domain.Repository = (*ArticleRepository)(nil)

// NewArticleRepository returns a repository bound to the articles collection of db.
func NewArticleRepository(db *mongo.Database) *ArticleRepository {
	return &ArticleRepository{coll: db.Collection(collectionName)}
}

// Save inserts a new article and writes the generated ID back onto the entity.
func (r *ArticleRepository) Save(ctx context.Context, a *domain.Article) error {
	id := bson.NewObjectID()
	doc := articleDoc{ID: id, Title: a.Title, Body: a.Body, CreatedAt: a.CreatedAt}

	if _, err := r.coll.InsertOne(ctx, doc); err != nil {
		return err
	}
	a.ID = id.Hex()

	return nil
}

// Update writes the mutable fields of an existing article.
func (r *ArticleRepository) Update(ctx context.Context, a *domain.Article) error {
	id, err := bson.ObjectIDFromHex(a.ID)
	if err != nil {
		return domain.ErrNotFound
	}

	res, err := r.coll.UpdateByID(ctx, id, bson.M{
		"$set": bson.M{"title": a.Title, "body": a.Body},
	})
	if err != nil {
		return err
	}
	if res.MatchedCount == 0 {
		return domain.ErrNotFound
	}

	return nil
}

// Delete removes an article by ID.
func (r *ArticleRepository) Delete(ctx context.Context, id string) error {
	oid, err := bson.ObjectIDFromHex(id)
	if err != nil {
		return domain.ErrNotFound
	}

	res, err := r.coll.DeleteOne(ctx, bson.M{"_id": oid})
	if err != nil {
		return err
	}
	if res.DeletedCount == 0 {
		return domain.ErrNotFound
	}

	return nil
}

// FindByID returns a single article. A malformed or unknown ID yields ErrNotFound.
func (r *ArticleRepository) FindByID(ctx context.Context, id string) (*domain.Article, error) {
	oid, err := bson.ObjectIDFromHex(id)
	if err != nil {
		return nil, domain.ErrNotFound
	}

	var doc articleDoc
	if err := r.coll.FindOne(ctx, bson.M{"_id": oid}).Decode(&doc); err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			return nil, domain.ErrNotFound
		}
		return nil, err
	}

	a := doc.toDomain()

	return &a, nil
}

// Paginate returns one newest-first page of articles and the total count.
func (r *ArticleRepository) Paginate(ctx context.Context, page, perPage int) ([]domain.Article, int64, error) {
	total, err := r.coll.CountDocuments(ctx, bson.M{})
	if err != nil {
		return nil, 0, err
	}

	opts := options.Find().
		SetSort(bson.D{{Key: "created_at", Value: -1}, {Key: "_id", Value: -1}}).
		SetSkip(int64((page - 1) * perPage)).
		SetLimit(int64(perPage))

	cur, err := r.coll.Find(ctx, bson.M{}, opts)
	if err != nil {
		return nil, 0, err
	}
	defer cur.Close(ctx)

	var docs []articleDoc
	if err := cur.All(ctx, &docs); err != nil {
		return nil, 0, err
	}

	articles := make([]domain.Article, len(docs))
	for i, d := range docs {
		articles[i] = d.toDomain()
	}

	return articles, total, nil
}
