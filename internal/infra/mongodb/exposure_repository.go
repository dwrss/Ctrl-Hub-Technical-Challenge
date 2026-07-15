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
	A8         float64      `bson:"a8"`
	Points     float64      `bson:"points"`
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
		A8:         e.A8(),
		Points:     e.Points(),
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

// summaryAccumulatorDoc mirrors the $group stage's output shape in
// SummarizeByUser's pipeline.
type summaryAccumulatorDoc struct {
	PointsTotal    float64 `bson:"points_total"`
	A8SquaredTotal float64 `bson:"a8_squared_total"`
}

// SummarizeByUser computes a user's exposure totals via an MongoDB
// aggregation pipeline.
// $group performs the summation server-side;
// a8 is squared per-document ($pow) before summing so the caller only needs
// to take the final sqrt (see domain.FinalizeExposureSummary).
func (r *ExposureRepository) SummarizeByUser(ctx context.Context, userID uuid.UUID, from, to *time.Time) (domain.ExposureAccumulator, error) {
	match := bson.M{"user_id": userID.String()}

	occurredAt := bson.M{}
	if from != nil {
		occurredAt["$gte"] = *from
	}
	if to != nil {
		occurredAt["$lte"] = *to
	}
	if len(occurredAt) > 0 {
		match["occurred_at"] = occurredAt
	}

	pipeline := mongo.Pipeline{
		{{Key: "$match", Value: match}},
		{{Key: "$group", Value: bson.M{
			"_id":              nil,
			"points_total":     bson.M{"$sum": "$points"},
			"a8_squared_total": bson.M{"$sum": bson.M{"$pow": bson.A{"$a8", 2}}},
		}}},
	}

	cursor, err := r.collection.Aggregate(ctx, pipeline)
	if err != nil {
		return domain.ExposureAccumulator{}, err
	}
	defer func(cursor *mongo.Cursor, ctx context.Context) {
		err := cursor.Close(ctx)
		if err != nil {
			log.Printf("failed to close cursor: %v", err)
		}
	}(cursor, ctx)

	var docs []summaryAccumulatorDoc
	if err := cursor.All(ctx, &docs); err != nil {
		return domain.ExposureAccumulator{}, err
	}

	// $group over no matching documents yields no rows, not a zeroed
	// document — a user with no exposures in the window is a valid,
	// non-error case that must still report a zero accumulator.
	if len(docs) == 0 {
		return domain.ExposureAccumulator{}, nil
	}

	return domain.ExposureAccumulator{
		PointsTotal:    docs[0].PointsTotal,
		A8SquaredTotal: docs[0].A8SquaredTotal,
	}, nil
}
