package redis

import (
	"context"
	"strconv"

	"github.com/redis/go-redis/v9"

	"equipment-exposure-service/internal/app"
)

// EventPublisher publishes domain events onto a Redis Stream via XADD.
// Streams (rather than Pub/Sub) were chosen so events are durable and
// replayable by consumer groups added later, which fits an event-driven
// architecture better than fire-and-forget Pub/Sub.
type EventPublisher struct {
	client *redis.Client
	stream string
}

func NewEventPublisher(client *redis.Client, stream string) *EventPublisher {
	return &EventPublisher{client: client, stream: stream}
}

func (p *EventPublisher) Publish(ctx context.Context, event app.ExposureRecordedEvent) error {
	values := map[string]interface{}{
		"event_type":   "exposure.recorded",
		"exposure_id":  event.ExposureID.String(),
		"user_id":      event.UserID.String(),
		"equipment_id": event.EquipmentID.String(),
		"a8":           strconv.FormatFloat(event.A8, 'f', -1, 64),
		"points":       strconv.FormatFloat(event.Points, 'f', -1, 64),
		"occurred_at":  event.OccurredAt.Format(RFC3339Milli),
	}

	return p.client.XAdd(ctx, &redis.XAddArgs{
		Stream: p.stream,
		Values: values,
	}).Err()
}

// PublishOrphaned publishes a batch of ExposureOrphanedEvents onto the same
// stream as Publish (distinguished by event_type — a dedicated stream isn't
// warranted for this scaffold's scale), pipelining the XADDs so N events
// cost one round trip rather than N.
func (p *EventPublisher) PublishOrphaned(ctx context.Context, events []app.ExposureOrphanedEvent) error {
	if len(events) == 0 {
		return nil
	}

	pipe := p.client.Pipeline()
	for _, event := range events {
		values := map[string]interface{}{
			"event_type":  "exposure.orphaned",
			"exposure_id": event.ExposureID.String(),
			"user_id":     event.UserID.String(),
			"detected_at": event.DetectedAt.Format(RFC3339Milli),
		}
		pipe.XAdd(ctx, &redis.XAddArgs{
			Stream: p.stream,
			Values: values,
		})
	}

	_, err := pipe.Exec(ctx)
	return err
}

const RFC3339Milli = "2006-01-02T15:04:05.000Z07:00"
