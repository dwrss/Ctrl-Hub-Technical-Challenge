package domain

import (
	"context"
	"fmt"

	"github.com/google/uuid"
)

type EquipmentItem struct {
	id                 uuid.UUID
	name               string
	vibrationMagnitude float64
}

// NewEquipmentItem constructs an EquipmentItem, rejecting a non-positive
// vibration magnitude since it would make every exposure computed against
// this equipment meaningless (A8/Points would be zero or NaN-adjacent).
func NewEquipmentItem(id uuid.UUID, name string, vibrationMagnitude float64) (EquipmentItem, error) {
	if vibrationMagnitude <= 0 {
		return EquipmentItem{}, fmt.Errorf("%w: vibration magnitude must be positive, got %v", ErrInvalidInput, vibrationMagnitude)
	}
	return EquipmentItem{id: id, name: name, vibrationMagnitude: vibrationMagnitude}, nil
}

func (e EquipmentItem) ID() uuid.UUID               { return e.id }
func (e EquipmentItem) Name() string                { return e.name }
func (e EquipmentItem) VibrationMagnitude() float64 { return e.vibrationMagnitude }

type EquipmentRepository interface {
	Get(ctx context.Context, id uuid.UUID) (EquipmentItem, error)
}
