package app

import (
	"context"
	"time"

	"github.com/google/uuid"
)

// ExposureRecordedEvent is published whenever a new exposure is recorded
type ExposureRecordedEvent struct {
	ExposureID  uuid.UUID
	UserID      uuid.UUID
	EquipmentID uuid.UUID
	A8          float64
	Points      float64
	OccurredAt  time.Time
}

// ExposureOrphanedEvent is published when an exposure references a user
// that can no longer be resolved (deleted or corrupted data), indicating a
// data-integrity issue. It fires on read (whenever the orphan is encountered
// while serving a request).
type ExposureOrphanedEvent struct {
	ExposureID uuid.UUID
	UserID     uuid.UUID
	DetectedAt time.Time
}

// EventPublisher is the outbound port for publishing domain events. It is
// implemented by an infra package (e.g. Redis Streams).
type EventPublisher interface {
	Publish(ctx context.Context, event ExposureRecordedEvent) error
	// PublishOrphaned publishes a batch of orphaned-exposure events in a
	// single round trip.
	// An empty slice is a no-op.
	PublishOrphaned(ctx context.Context, events []ExposureOrphanedEvent) error
}
