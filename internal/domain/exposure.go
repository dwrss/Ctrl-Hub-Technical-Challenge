package domain

import (
	"context"
	"fmt"
	"math"
	"time"

	"github.com/google/uuid"
)

type Exposure struct {
	id         uuid.UUID
	equipment  EquipmentItem
	userID     uuid.UUID
	duration   Minutes
	occurredAt time.Time
}

// NewExposure constructs an Exposure, rejecting a non-positive duration or vibration magnitude
//
//	since this would make the record pointless.
func NewExposure(id uuid.UUID, equipment EquipmentItem, userID uuid.UUID, duration Minutes, occurredAt time.Time) (Exposure, error) {
	if duration.Int() <= 0 {
		return Exposure{}, fmt.Errorf("%w: duration must be positive, got %d", ErrInvalidInput, duration.Int())
	}
	if equipment.VibrationMagnitude() <= 0 {
		return Exposure{}, fmt.Errorf("%w: equipment must have a positive vibration magnitude", ErrInvalidInput)
	}

	return Exposure{
		id:         id,
		equipment:  equipment,
		userID:     userID,
		duration:   duration,
		occurredAt: occurredAt,
	}, nil
}

func (e Exposure) ID() uuid.UUID            { return e.id }
func (e Exposure) Equipment() EquipmentItem { return e.equipment }
func (e Exposure) UserID() uuid.UUID        { return e.userID }
func (e Exposure) Duration() Minutes        { return e.duration }
func (e Exposure) OccurredAt() time.Time    { return e.occurredAt }

// A8 returns the Partial Exposure A(8) for this exposure, per the HSE HAVS
// methodology: vibration magnitude scaled by the square root of the fraction
// of an 8-hour reference day the equipment was used.
func (e Exposure) A8() float64 {
	return e.equipment.VibrationMagnitude() * math.Sqrt(e.duration.dailyFraction())
}

// Points returns the Partial Exposure Points for this exposure. By
// construction Points == 16 * A8() * A8() for any single exposure; see
// SummarizeExposures for why that identity matters when aggregating.
func (e Exposure) Points() float64 {
	points := math.Pow(e.equipment.VibrationMagnitude()/2.5, 2) * (e.duration.dailyFraction() * 100)
	return math.Round(points)
}

// ExposureRepository is the persistence port for exposures, implemented by
// an infra package (e.g. MongoDB) and consumed by the application layer.
type ExposureRepository interface {
	List(ctx context.Context) ([]Exposure, error)
	Get(ctx context.Context, id uuid.UUID) (Exposure, error)
	Create(ctx context.Context, e Exposure) (Exposure, error)
	// ListByUser returns a user's exposures. A nil from/to means that bound
	// is unset, i.e. no windowing on that side.
	ListByUser(ctx context.Context, userID uuid.UUID, from, to *time.Time) ([]Exposure, error)
}
