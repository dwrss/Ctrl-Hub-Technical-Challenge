package http

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"

	"equipment-exposure-service/internal/app"
	"equipment-exposure-service/internal/domain"
)

// mustExposure builds a domain.Exposure via the real constructor, failing
// the test immediately if the inputs are invalid, so test fixtures don't
// need to thread errors through every seed call.
func mustExposure(t *testing.T, id uuid.UUID, equipment domain.EquipmentItem, userID uuid.UUID, duration domain.Minutes, occurredAt time.Time) domain.Exposure {
	t.Helper()
	e, err := domain.NewExposure(id, equipment, userID, duration, occurredAt)
	if err != nil {
		t.Fatalf("NewExposure: %v", err)
	}
	return e
}

// fakeExposureRepo, fakeEquipmentRepo, fakeUserRepo, and fakePublisher are
// in-memory doubles for the ports consumed by ExposureService. They exist
// to exercise the router -> handler -> service -> DTO path end-to-end
// without a live Mongo/Redis stack, and are shared across this package's
// handler tests.

type fakeExposureRepo struct {
	byID  map[uuid.UUID]domain.Exposure
	items []domain.Exposure
}

func newFakeExposureRepo() *fakeExposureRepo {
	return &fakeExposureRepo{byID: map[uuid.UUID]domain.Exposure{}}
}

func (r *fakeExposureRepo) List(ctx context.Context) ([]domain.Exposure, error) {
	return r.items, nil
}

func (r *fakeExposureRepo) Get(ctx context.Context, id uuid.UUID) (domain.Exposure, error) {
	e, ok := r.byID[id]
	if !ok {
		return domain.Exposure{}, domain.ErrNotFound
	}
	return e, nil
}

func (r *fakeExposureRepo) Create(ctx context.Context, e domain.Exposure) (domain.Exposure, error) {
	r.byID[e.ID()] = e
	r.items = append(r.items, e)
	return e, nil
}

func (r *fakeExposureRepo) ListByUser(ctx context.Context, userID uuid.UUID, from, to *time.Time) ([]domain.Exposure, error) {
	var out []domain.Exposure
	for _, e := range r.items {
		if e.UserID() != userID {
			continue
		}
		if from != nil && e.OccurredAt().Before(*from) {
			continue
		}
		if to != nil && e.OccurredAt().After(*to) {
			continue
		}
		out = append(out, e)
	}
	return out, nil
}

type fakeEquipmentRepo struct {
	byID map[uuid.UUID]domain.EquipmentItem
}

func (r *fakeEquipmentRepo) Get(ctx context.Context, id uuid.UUID) (domain.EquipmentItem, error) {
	e, ok := r.byID[id]
	if !ok {
		return domain.EquipmentItem{}, domain.ErrNotFound
	}
	return e, nil
}

type fakeUserRepo struct {
	byID         map[uuid.UUID]domain.User
	getCalls     int
	getManyCalls int
}

func (r *fakeUserRepo) Get(ctx context.Context, id uuid.UUID) (domain.User, error) {
	r.getCalls++
	u, ok := r.byID[id]
	if !ok {
		return domain.User{}, domain.ErrNotFound
	}
	return u, nil
}

func (r *fakeUserRepo) GetMany(ctx context.Context, ids []uuid.UUID) (map[uuid.UUID]domain.User, error) {
	r.getManyCalls++
	users := make(map[uuid.UUID]domain.User, len(ids))
	for _, id := range ids {
		if u, ok := r.byID[id]; ok {
			users[id] = u
		}
	}
	return users, nil
}

type fakePublisher struct {
	published            []app.ExposureRecordedEvent
	publishedOrphaned    []app.ExposureOrphanedEvent
	publishOrphanedCalls int
	orphanedErr          error // if set, PublishOrphaned returns this error instead of recording
}

func (p *fakePublisher) Publish(ctx context.Context, event app.ExposureRecordedEvent) error {
	p.published = append(p.published, event)
	return nil
}

func (p *fakePublisher) PublishOrphaned(ctx context.Context, events []app.ExposureOrphanedEvent) error {
	p.publishOrphanedCalls++
	if p.orphanedErr != nil {
		return p.orphanedErr
	}
	p.publishedOrphaned = append(p.publishedOrphaned, events...)
	return nil
}
