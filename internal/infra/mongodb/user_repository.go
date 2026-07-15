package mongodb

import (
	"context"
	"errors"
	"log"

	"github.com/google/uuid"
	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"

	"equipment-exposure-service/internal/domain"
)

const userCollection = "users"

type userDoc struct {
	ID   string `bson:"_id"`
	Name string `bson:"name"`
}

type UserRepository struct {
	collection *mongo.Collection
}

func NewUserRepository(db *mongo.Database) *UserRepository {
	return &UserRepository{collection: db.Collection(userCollection)}
}

func (r *UserRepository) Get(ctx context.Context, id uuid.UUID) (domain.User, error) {
	var doc userDoc
	err := r.collection.FindOne(ctx, bson.M{"_id": id.String()}).Decode(&doc)
	if errors.Is(err, mongo.ErrNoDocuments) {
		return domain.User{}, domain.ErrNotFound
	}
	if err != nil {
		return domain.User{}, err
	}

	return domain.NewUser(uuid.MustParse(doc.ID), doc.Name)
}

func (r *UserRepository) GetMany(ctx context.Context, ids []uuid.UUID) (map[uuid.UUID]domain.User, error) {
	idStrings := make([]string, len(ids))
	for i, id := range ids {
		idStrings[i] = id.String()
	}

	cursor, err := r.collection.Find(ctx, bson.M{"_id": bson.M{"$in": idStrings}})
	if err != nil {
		return nil, err
	}
	defer func(cursor *mongo.Cursor, ctx context.Context) {
		err := cursor.Close(ctx)
		if err != nil {
			log.Printf("failed to close cursor: %v", err)
		}
	}(cursor, ctx)

	var docs []userDoc
	if err := cursor.All(ctx, &docs); err != nil {
		return nil, err
	}

	users := make(map[uuid.UUID]domain.User, len(docs))
	for _, doc := range docs {
		user, err := domain.NewUser(uuid.MustParse(doc.ID), doc.Name)
		if err != nil {
			return nil, err
		}
		users[user.ID()] = user
	}
	return users, nil
}
