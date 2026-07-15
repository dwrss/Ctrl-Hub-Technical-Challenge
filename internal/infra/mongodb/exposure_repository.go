package mongodb

import (
	"context"
	"errors"
	"log"
	"time"

	"github.com/google/uuid"
	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"

	"equipment-exposure-service/internal/domain"
)

const exposureCollection = "exposures"

type exposureDoc struct {
	ID         string       `bson:"_id"`
	Equipment  equipmentDoc `bson:"equipment"`
	UserID     string       `bson:"user_id"`
	Duration   int          `bson:"duration"`
	OccurredAt time.Time    `bson:"occurred_at"`
}

type ExposureRepository struct {
	collection *mongo.Collection
}

func NewExposureRepository(db *mongo.Database) *ExposureRepository {
	return &ExposureRepository{collection: db.Collection(exposureCollection)}
}

func toExposureDoc(e domain.Exposure) exposureDoc {
	return exposureDoc{
		ID: e.ID().String(),
		Equipment: equipmentDoc{
			ID:                 e.Equipment().ID().String(),
			Name:               e.Equipment().Name(),
			VibrationMagnitude: e.Equipment().VibrationMagnitude(),
		},
		UserID:     e.UserID().String(),
		Duration:   e.Duration().Int(),
		OccurredAt: e.OccurredAt(),
	}
}

// fromExposureDoc reconstructs the domain aggregate from a persisted
// document. It re-runs the domain constructors, so a document that somehow
// violates a domain invariant (e.g. duration <= 0) is surfaced as an error
// here rather than silently producing an invalid Exposure.
func fromExposureDoc(doc exposureDoc) (domain.Exposure, error) {
	equipment, err := domain.NewEquipmentItem(uuid.MustParse(doc.Equipment.ID), doc.Equipment.Name, doc.Equipment.VibrationMagnitude)
	if err != nil {
		return domain.Exposure{}, err
	}

	duration, err := domain.NewMinutes(doc.Duration)
	if err != nil {
		return domain.Exposure{}, err
	}

	return domain.NewExposure(uuid.MustParse(doc.ID), equipment, uuid.MustParse(doc.UserID), duration, doc.OccurredAt)
}

func (r *ExposureRepository) List(ctx context.Context) ([]domain.Exposure, error) {
	cursor, err := r.collection.Find(ctx, bson.M{})
	if err != nil {
		return nil, err
	}
	defer func(cursor *mongo.Cursor, ctx context.Context) {
		err := cursor.Close(ctx)
		if err != nil {
			log.Printf("failed to close cursor: %v", err)
		}
	}(cursor, ctx)

	var docs []exposureDoc
	if err := cursor.All(ctx, &docs); err != nil {
		return nil, err
	}

	exposures := make([]domain.Exposure, 0, len(docs))
	for _, doc := range docs {
		exposure, err := fromExposureDoc(doc)
		if err != nil {
			return nil, err
		}
		exposures = append(exposures, exposure)
	}
	return exposures, nil
}

func (r *ExposureRepository) Get(ctx context.Context, id uuid.UUID) (domain.Exposure, error) {
	var doc exposureDoc
	err := r.collection.FindOne(ctx, bson.M{"_id": id.String()}).Decode(&doc)
	if errors.Is(err, mongo.ErrNoDocuments) {
		return domain.Exposure{}, domain.ErrNotFound
	}
	if err != nil {
		return domain.Exposure{}, err
	}
	return fromExposureDoc(doc)
}

func (r *ExposureRepository) Create(ctx context.Context, e domain.Exposure) (domain.Exposure, error) {
	doc := toExposureDoc(e)
	if _, err := r.collection.InsertOne(ctx, doc); err != nil {
		return domain.Exposure{}, err
	}
	return e, nil
}

func (r *ExposureRepository) ListByUser(ctx context.Context, userID uuid.UUID, from, to *time.Time) ([]domain.Exposure, error) {
	filter := bson.M{"user_id": userID.String()}

	occurredAt := bson.M{}
	if from != nil {
		occurredAt["$gte"] = *from
	}
	if to != nil {
		occurredAt["$lte"] = *to
	}
	if len(occurredAt) > 0 {
		filter["occurred_at"] = occurredAt
	}

	cursor, err := r.collection.Find(ctx, filter)
	if err != nil {
		return nil, err
	}
	defer func(cursor *mongo.Cursor, ctx context.Context) {
		err := cursor.Close(ctx)
		if err != nil {
			log.Printf("failed to close cursor: %v", err)
		}
	}(cursor, ctx)

	var docs []exposureDoc
	if err := cursor.All(ctx, &docs); err != nil {
		return nil, err
	}

	exposures := make([]domain.Exposure, 0, len(docs))
	for _, doc := range docs {
		exposure, err := fromExposureDoc(doc)
		if err != nil {
			return nil, err
		}
		exposures = append(exposures, exposure)
	}
	return exposures, nil
}
