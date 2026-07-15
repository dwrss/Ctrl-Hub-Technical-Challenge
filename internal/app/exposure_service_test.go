package app

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"

	"equipment-exposure-service/internal/domain"
)

// Fakes mirror the minimal in-memory doubles used in internal/http's tests,
// duplicated here (rather than shared) since those are package-scoped
// _test.go files in a different package.

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

func (r *fakeExposureRepo) SummarizeByUser(ctx context.Context, userID uuid.UUID, from, to *time.Time) (domain.ExposureAccumulator, error) {
	var acc domain.ExposureAccumulator
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
		acc = acc.Add(e)
	}
	return acc, nil
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
	byID map[uuid.UUID]domain.User
}

func (r *fakeUserRepo) Get(ctx context.Context, id uuid.UUID) (domain.User, error) {
	u, ok := r.byID[id]
	if !ok {
		return domain.User{}, domain.ErrNotFound
	}
	return u, nil
}

func (r *fakeUserRepo) GetMany(ctx context.Context, ids []uuid.UUID) (map[uuid.UUID]domain.User, error) {
	users := make(map[uuid.UUID]domain.User, len(ids))
	for _, id := range ids {
		if u, ok := r.byID[id]; ok {
			users[id] = u
		}
	}
	return users, nil
}

type fakePublisher struct {
	published         []ExposureRecordedEvent
	publishedOrphaned []ExposureOrphanedEvent
	publishErr        error
}

func (p *fakePublisher) Publish(ctx context.Context, event ExposureRecordedEvent) error {
	if p.publishErr != nil {
		return p.publishErr
	}
	p.published = append(p.published, event)
	return nil
}

func (p *fakePublisher) PublishOrphaned(ctx context.Context, events []ExposureOrphanedEvent) error {
	p.publishedOrphaned = append(p.publishedOrphaned, events...)
	return nil
}

func testFixtures(t *testing.T) (domain.EquipmentItem, domain.User) {
	t.Helper()
	equipment, err := domain.NewEquipmentItem(uuid.MustParse("2e85d43d-dd9b-4e8d-b2ce-97b8d7d69d49"), "AirCat - Drill - 4337", 2.1)
	if err != nil {
		t.Fatalf("NewEquipmentItem: %v", err)
	}
	user, err := domain.NewUser(uuid.MustParse("713be58e-0d79-4df2-a85c-9f44ca513a7d"), "Bobby Tables")
	if err != nil {
		t.Fatalf("NewUser: %v", err)
	}
	return equipment, user
}

// TestRecordExposure_PublishesEventMatchingCreatedExposure asserts that the
// ExposureRecordedEvent published on success actually carries the same
// A8/Points/OccurredAt/IDs as the exposure that was persisted, not just that
// "an event was published".
func TestRecordExposure_PublishesEventMatchingCreatedExposure(t *testing.T) {
	equipment, user := testFixtures(t)
	exposureRepo := newFakeExposureRepo()
	equipmentRepo := &fakeEquipmentRepo{byID: map[uuid.UUID]domain.EquipmentItem{equipment.ID(): equipment}}
	userRepo := &fakeUserRepo{byID: map[uuid.UUID]domain.User{user.ID(): user}}
	publisher := &fakePublisher{}

	service := NewExposureService(exposureRepo, equipmentRepo, userRepo, publisher)

	view, err := service.RecordExposure(context.Background(), user.ID(), equipment.ID(), 120)
	if err != nil {
		t.Fatalf("RecordExposure: %v", err)
	}

	if len(publisher.published) != 1 {
		t.Fatalf("published events = %d, want 1", len(publisher.published))
	}
	event := publisher.published[0]
	created := view.Exposure

	if event.ExposureID != created.ID() {
		t.Errorf("event.ExposureID = %v, want %v", event.ExposureID, created.ID())
	}
	if event.UserID != created.UserID() {
		t.Errorf("event.UserID = %v, want %v", event.UserID, created.UserID())
	}
	if event.EquipmentID != created.Equipment().ID() {
		t.Errorf("event.EquipmentID = %v, want %v", event.EquipmentID, created.Equipment().ID())
	}
	if event.A8 != created.A8() {
		t.Errorf("event.A8 = %v, want %v", event.A8, created.A8())
	}
	if event.Points != created.Points() {
		t.Errorf("event.Points = %v, want %v", event.Points, created.Points())
	}
	if !event.OccurredAt.Equal(created.OccurredAt()) {
		t.Errorf("event.OccurredAt = %v, want %v", event.OccurredAt, created.OccurredAt())
	}
}

// TestRecordExposure_PublishFailureIsReturnedAfterPersisting pins the
// current behavior when the outbound publish fails: the exposure is already
// persisted (Create ran first), but RecordExposure still returns an error to
// the caller.
func TestRecordExposure_PublishFailureIsReturnedAfterPersisting(t *testing.T) {
	equipment, user := testFixtures(t)
	exposureRepo := newFakeExposureRepo()
	equipmentRepo := &fakeEquipmentRepo{byID: map[uuid.UUID]domain.EquipmentItem{equipment.ID(): equipment}}
	userRepo := &fakeUserRepo{byID: map[uuid.UUID]domain.User{user.ID(): user}}
	publisher := &fakePublisher{publishErr: errors.New("redis unavailable")}

	service := NewExposureService(exposureRepo, equipmentRepo, userRepo, publisher)

	_, err := service.RecordExposure(context.Background(), user.ID(), equipment.ID(), 120)
	if err == nil {
		t.Fatal("RecordExposure: want error when publish fails, got nil")
	}

	if len(exposureRepo.items) != 1 {
		t.Errorf("persisted exposures = %d, want 1 (exposure is created before publish is attempted)", len(exposureRepo.items))
	}
}

// TestGetExposure_OrphanedUserReturnsDanglingReference pins the service's
// own contract for an exposure whose user can't be resolved: ErrDanglingReference,
// not the underlying ErrNotFound, and one orphaned event published.
func TestGetExposure_OrphanedUserReturnsDanglingReference(t *testing.T) {
	equipment, _ := testFixtures(t)
	orphanedUserID := uuid.New()
	duration, err := domain.NewMinutes(5)
	if err != nil {
		t.Fatalf("NewMinutes: %v", err)
	}
	exposure, err := domain.NewExposure(uuid.New(), equipment, orphanedUserID, duration, time.Now())
	if err != nil {
		t.Fatalf("NewExposure: %v", err)
	}

	exposureRepo := newFakeExposureRepo()
	if _, err := exposureRepo.Create(context.Background(), exposure); err != nil {
		t.Fatalf("seed exposure: %v", err)
	}
	equipmentRepo := &fakeEquipmentRepo{byID: map[uuid.UUID]domain.EquipmentItem{equipment.ID(): equipment}}
	userRepo := &fakeUserRepo{byID: map[uuid.UUID]domain.User{}} // orphanedUserID deliberately absent
	publisher := &fakePublisher{}

	service := NewExposureService(exposureRepo, equipmentRepo, userRepo, publisher)

	_, err = service.GetExposure(context.Background(), exposure.ID())
	if !errors.Is(err, domain.ErrDanglingReference) {
		t.Errorf("GetExposure err = %v, want ErrDanglingReference", err)
	}
	if len(publisher.publishedOrphaned) != 1 {
		t.Errorf("published orphaned events = %d, want 1", len(publisher.publishedOrphaned))
	}
}

// TestGetUserExposureSummary_FinalizesAccumulatorFromRepository asserts the
// summary path calls through to SummarizeByUser and finalizes it correctly,
// directly against the domain.ExposureSummary rather than round-tripping
// through JSON as the HTTP-layer tests do.
func TestGetUserExposureSummary_FinalizesAccumulatorFromRepository(t *testing.T) {
	equipment, user := testFixtures(t)
	duration, err := domain.NewMinutes(120)
	if err != nil {
		t.Fatalf("NewMinutes: %v", err)
	}
	exposure, err := domain.NewExposure(uuid.New(), equipment, user.ID(), duration, time.Now())
	if err != nil {
		t.Fatalf("NewExposure: %v", err)
	}

	exposureRepo := newFakeExposureRepo()
	if _, err := exposureRepo.Create(context.Background(), exposure); err != nil {
		t.Fatalf("seed exposure: %v", err)
	}
	equipmentRepo := &fakeEquipmentRepo{byID: map[uuid.UUID]domain.EquipmentItem{equipment.ID(): equipment}}
	userRepo := &fakeUserRepo{byID: map[uuid.UUID]domain.User{user.ID(): user}}
	publisher := &fakePublisher{}

	service := NewExposureService(exposureRepo, equipmentRepo, userRepo, publisher)

	summary, err := service.GetUserExposureSummary(context.Background(), user.ID(), nil, nil)
	if err != nil {
		t.Fatalf("GetUserExposureSummary: %v", err)
	}
	if summary.A8() != exposure.A8() {
		t.Errorf("summary.A8() = %v, want %v", summary.A8(), exposure.A8())
	}
	if summary.Points() != exposure.Points() {
		t.Errorf("summary.Points() = %v, want %v", summary.Points(), exposure.Points())
	}
}

// TestGetUserExposureSummary_UnknownUserReturnsNotFound asserts the summary
// path 404s (via ErrNotFound) for an unknown user rather than returning a
// zero-value summary, which would be indistinguishable from "user exists but
// has no exposures".
func TestGetUserExposureSummary_UnknownUserReturnsNotFound(t *testing.T) {
	exposureRepo := newFakeExposureRepo()
	equipmentRepo := &fakeEquipmentRepo{byID: map[uuid.UUID]domain.EquipmentItem{}}
	userRepo := &fakeUserRepo{byID: map[uuid.UUID]domain.User{}}
	publisher := &fakePublisher{}

	service := NewExposureService(exposureRepo, equipmentRepo, userRepo, publisher)

	_, err := service.GetUserExposureSummary(context.Background(), uuid.New(), nil, nil)
	if !errors.Is(err, domain.ErrNotFound) {
		t.Errorf("GetUserExposureSummary err = %v, want ErrNotFound", err)
	}
}
