package mongodb

import (
	"context"
	"errors"

	"github.com/google/uuid"
	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"

	"equipment-exposure-service/internal/domain"
)

const equipmentCollection = "equipment"

type equipmentDoc struct {
	ID                 string  `bson:"_id"`
	Name               string  `bson:"name"`
	VibrationMagnitude float64 `bson:"vibration_magnitude"`
}

type EquipmentRepository struct {
	collection *mongo.Collection
}

func NewEquipmentRepository(db *mongo.Database) *EquipmentRepository {
	return &EquipmentRepository{collection: db.Collection(equipmentCollection)}
}

func (r *EquipmentRepository) Get(ctx context.Context, id uuid.UUID) (domain.EquipmentItem, error) {
	var doc equipmentDoc
	err := r.collection.FindOne(ctx, bson.M{"_id": id.String()}).Decode(&doc)
	if errors.Is(err, mongo.ErrNoDocuments) {
		return domain.EquipmentItem{}, domain.ErrNotFound
	}
	if err != nil {
		return domain.EquipmentItem{}, err
	}

	return domain.NewEquipmentItem(uuid.MustParse(doc.ID), doc.Name, doc.VibrationMagnitude)
}
