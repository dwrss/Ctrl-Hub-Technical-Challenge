package mongodb

import (
	"context"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

// Seed idempotently upserts the equipment and user fixtures used as
// examples throughout spec.yaml, so requests copied directly from the spec
// work against a freshly started stack. Safe to call on every startup.
func Seed(ctx context.Context, db *mongo.Database) error {
	equipment := []equipmentDoc{
		{
			ID:                 "2e85d43d-dd9b-4e8d-b2ce-97b8d7d69d49",
			Name:               "AirCat - Drill - 4337",
			VibrationMagnitude: 2.1,
		},
		{
			ID:                 "36603447-2f30-41b1-a908-526c0b6f1755",
			Name:               "JCB - Hydraulic Breaker - CEJCBHM25",
			VibrationMagnitude: 4.0,
		},
	}

	equipmentCol := db.Collection(equipmentCollection)
	for _, item := range equipment {
		if _, err := equipmentCol.ReplaceOne(
			ctx,
			bson.M{"_id": item.ID},
			item,
			options.Replace().SetUpsert(true),
		); err != nil {
			return err
		}
	}

	users := []userDoc{
		{
			ID:   "713be58e-0d79-4df2-a85c-9f44ca513a7d",
			Name: "Bobby Tables",
		},
		{
			ID:   "b3a0eddc-e20d-453b-893e-36794a1daffe",
			Name: "Ada Lovelace",
		},
		{
			ID:   "78776e50-0e1a-4282-ba37-83d54c1b4795",
			Name: "Grace Hopper",
		},
	}

	userCol := db.Collection(userCollection)
	for _, u := range users {
		if _, err := userCol.ReplaceOne(
			ctx,
			bson.M{"_id": u.ID},
			u,
			options.Replace().SetUpsert(true),
		); err != nil {
			return err
		}
	}

	return nil
}
