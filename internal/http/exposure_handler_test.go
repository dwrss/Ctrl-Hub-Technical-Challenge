package http

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/google/uuid"

	"equipment-exposure-service/internal/app"
	"equipment-exposure-service/internal/domain"
)

func TestExposureEndpoints(t *testing.T) {
	equipment, err := domain.NewEquipmentItem(uuid.MustParse("2e85d43d-dd9b-4e8d-b2ce-97b8d7d69d49"), "AirCat - Drill - 4337", 2.1)
	if err != nil {
		t.Fatalf("NewEquipmentItem: %v", err)
	}
	user, err := domain.NewUser(uuid.MustParse("713be58e-0d79-4df2-a85c-9f44ca513a7d"), "Bobby Tables")
	if err != nil {
		t.Fatalf("NewUser: %v", err)
	}

	exposureRepo := newFakeExposureRepo()
	equipmentRepo := &fakeEquipmentRepo{byID: map[uuid.UUID]domain.EquipmentItem{equipment.ID(): equipment}}
	userRepo := &fakeUserRepo{byID: map[uuid.UUID]domain.User{user.ID(): user}}
	publisher := &fakePublisher{}

	service := app.NewExposureService(exposureRepo, equipmentRepo, userRepo, publisher)
	router := NewRouter(NewExposureHandler(service), NewExposureSummaryHandler(service))

	// POST /exposure with a valid body should return 201 and a body whose
	// JSON field names match spec.yaml's snake_case contract.
	body, _ := json.Marshal(recordExposureRequest{
		EquipmentID: equipment.ID().String(),
		UserID:      user.ID().String(),
		Duration:    5,
	})
	req := httptest.NewRequest("POST", "/exposure", bytes.NewReader(body))
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != 201 {
		t.Fatalf("POST /exposure: status = %d, body = %s", rec.Code, rec.Body.String())
	}

	var created exposureResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &created); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if created.Equipment.VibrationMagnitude != 2.1 {
		t.Errorf("equipment.vibration_magnitude = %v, want 2.1 (json tag mismatch?)", created.Equipment.VibrationMagnitude)
	}
	if created.User.Name != "Bobby Tables" {
		t.Errorf("user.name = %q, want %q", created.User.Name, "Bobby Tables")
	}
	if created.Duration != 5 {
		t.Errorf("duration = %d, want 5", created.Duration)
	}
	if len(publisher.published) != 1 {
		t.Fatalf("expected 1 published event, got %d", len(publisher.published))
	}

	createdID := created.ID

	// GET /exposure should list it.
	req = httptest.NewRequest("GET", "/exposure", nil)
	rec = httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != 200 {
		t.Fatalf("GET /exposure: status = %d", rec.Code)
	}
	var list []exposureResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &list); err != nil {
		t.Fatalf("decode list: %v", err)
	}
	if len(list) != 1 {
		t.Fatalf("expected 1 exposure in list, got %d", len(list))
	}
	if list[0].User.Name != "Bobby Tables" {
		t.Errorf("list[0].user.name = %q, want %q (list response must resolve the user, not just the exposure)", list[0].User.Name, "Bobby Tables")
	}

	// GET /exposure/{id} should return 200 (spec.yaml says 201, deliberate deviation).
	req = httptest.NewRequest("GET", "/exposure/"+createdID, nil)
	rec = httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != 200 {
		t.Fatalf("GET /exposure/{id}: status = %d, want 200", rec.Code)
	}

	// GET /exposure/{unknown-id} should 404.
	req = httptest.NewRequest("GET", "/exposure/"+uuid.New().String(), nil)
	rec = httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != 404 {
		t.Fatalf("GET /exposure/{unknown}: status = %d, want 404", rec.Code)
	}

	// GET /exposure/{invalid-uuid} should 400.
	req = httptest.NewRequest("GET", "/exposure/not-a-uuid", nil)
	rec = httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != 400 {
		t.Fatalf("GET /exposure/{invalid}: status = %d, want 400", rec.Code)
	}
}

// TestListExposures_BatchesUserLookups asserts that GET /exposure resolves
// the nested "user" field via a single UserRepository.GetMany call
// regardless of how many exposures are returned, rather than one Get call
// per exposure.
func TestListExposures_BatchesUserLookups(t *testing.T) {
	equipment, err := domain.NewEquipmentItem(uuid.MustParse("2e85d43d-dd9b-4e8d-b2ce-97b8d7d69d49"), "AirCat - Drill - 4337", 2.1)
	if err != nil {
		t.Fatalf("NewEquipmentItem: %v", err)
	}
	userA, err := domain.NewUser(uuid.New(), "User A")
	if err != nil {
		t.Fatalf("NewUser: %v", err)
	}
	userB, err := domain.NewUser(uuid.New(), "User B")
	if err != nil {
		t.Fatalf("NewUser: %v", err)
	}
	duration, err := domain.NewMinutes(5)
	if err != nil {
		t.Fatalf("NewMinutes: %v", err)
	}

	exposureRepo := newFakeExposureRepo()
	// Three exposures across two distinct users: enough to prove reuse
	// (userA appears twice) without a single-exposure list masking N+1.
	for _, userID := range []uuid.UUID{userA.ID(), userA.ID(), userB.ID()} {
		if _, err := exposureRepo.Create(context.Background(), mustExposure(t, uuid.New(), equipment, userID, duration, time.Now())); err != nil {
			t.Fatalf("seed exposure: %v", err)
		}
	}

	equipmentRepo := &fakeEquipmentRepo{byID: map[uuid.UUID]domain.EquipmentItem{equipment.ID(): equipment}}
	userRepo := &fakeUserRepo{byID: map[uuid.UUID]domain.User{userA.ID(): userA, userB.ID(): userB}}
	publisher := &fakePublisher{}

	service := app.NewExposureService(exposureRepo, equipmentRepo, userRepo, publisher)
	router := NewRouter(NewExposureHandler(service), NewExposureSummaryHandler(service))

	req := httptest.NewRequest("GET", "/exposure", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != 200 {
		t.Fatalf("GET /exposure: status = %d, body = %s", rec.Code, rec.Body.String())
	}

	var list []exposureResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &list); err != nil {
		t.Fatalf("decode list: %v", err)
	}
	if len(list) != 3 {
		t.Fatalf("expected 3 exposures in list, got %d", len(list))
	}

	if userRepo.getManyCalls != 1 {
		t.Errorf("GetMany calls = %d, want 1 (should batch, not call once per exposure)", userRepo.getManyCalls)
	}
	if userRepo.getCalls != 0 {
		t.Errorf("Get calls = %d, want 0 (list should not fall back to per-exposure lookups)", userRepo.getCalls)
	}
}

// TestListExposures_ExcludesOrphanedExposure asserts that an exposure whose
// user can't be resolved (e.g. deleted or corrupted data) is excluded from
// the list rather than failing the entire GET /exposure request with a 500.
func TestListExposures_ExcludesOrphanedExposure(t *testing.T) {
	equipment, err := domain.NewEquipmentItem(uuid.MustParse("2e85d43d-dd9b-4e8d-b2ce-97b8d7d69d49"), "AirCat - Drill - 4337", 2.1)
	if err != nil {
		t.Fatalf("NewEquipmentItem: %v", err)
	}
	knownUser, err := domain.NewUser(uuid.New(), "Known User")
	if err != nil {
		t.Fatalf("NewUser: %v", err)
	}
	orphanedUserID := uuid.New() // deliberately not present in userRepo
	duration, err := domain.NewMinutes(5)
	if err != nil {
		t.Fatalf("NewMinutes: %v", err)
	}

	exposureRepo := newFakeExposureRepo()
	if _, err := exposureRepo.Create(context.Background(), mustExposure(t, uuid.New(), equipment, knownUser.ID(), duration, time.Now())); err != nil {
		t.Fatalf("seed knownUser exposure: %v", err)
	}
	if _, err := exposureRepo.Create(context.Background(), mustExposure(t, uuid.New(), equipment, orphanedUserID, duration, time.Now())); err != nil {
		t.Fatalf("seed orphaned exposure: %v", err)
	}

	equipmentRepo := &fakeEquipmentRepo{byID: map[uuid.UUID]domain.EquipmentItem{equipment.ID(): equipment}}
	userRepo := &fakeUserRepo{byID: map[uuid.UUID]domain.User{knownUser.ID(): knownUser}}
	publisher := &fakePublisher{}

	service := app.NewExposureService(exposureRepo, equipmentRepo, userRepo, publisher)
	router := NewRouter(NewExposureHandler(service), NewExposureSummaryHandler(service))

	req := httptest.NewRequest("GET", "/exposure", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != 200 {
		t.Fatalf("GET /exposure: status = %d, body = %s (a dangling user reference should not fail the whole list)", rec.Code, rec.Body.String())
	}

	var list []exposureResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &list); err != nil {
		t.Fatalf("decode list: %v", err)
	}
	if len(list) != 1 {
		t.Fatalf("expected 1 resolvable exposure in list (orphan excluded), got %d", len(list))
	}
	if list[0].User.Name != "Known User" {
		t.Errorf("list[0].user.name = %q, want %q", list[0].User.Name, "Known User")
	}

	if len(publisher.publishedOrphaned) != 1 {
		t.Fatalf("expected 1 published orphaned-exposure event, got %d", len(publisher.publishedOrphaned))
	}
	if publisher.publishedOrphaned[0].UserID != orphanedUserID {
		t.Errorf("published event UserID = %v, want %v", publisher.publishedOrphaned[0].UserID, orphanedUserID)
	}
}

// TestListExposures_BatchesOrphanedEventPublish asserts that multiple
// orphaned exposures in a single list request are reported via ONE
// PublishOrphaned call carrying all of them, not one call per orphan.
// A single-orphan test can't distinguish batched from per-item publishing;
// this needs at least two.
func TestListExposures_BatchesOrphanedEventPublish(t *testing.T) {
	equipment, err := domain.NewEquipmentItem(uuid.MustParse("2e85d43d-dd9b-4e8d-b2ce-97b8d7d69d49"), "AirCat - Drill - 4337", 2.1)
	if err != nil {
		t.Fatalf("NewEquipmentItem: %v", err)
	}
	knownUser, err := domain.NewUser(uuid.New(), "Known User")
	if err != nil {
		t.Fatalf("NewUser: %v", err)
	}
	orphanedUserA := uuid.New()
	orphanedUserB := uuid.New()
	duration, err := domain.NewMinutes(5)
	if err != nil {
		t.Fatalf("NewMinutes: %v", err)
	}

	exposureRepo := newFakeExposureRepo()
	for _, userID := range []uuid.UUID{knownUser.ID(), orphanedUserA, orphanedUserB} {
		if _, err := exposureRepo.Create(context.Background(), mustExposure(t, uuid.New(), equipment, userID, duration, time.Now())); err != nil {
			t.Fatalf("seed exposure: %v", err)
		}
	}

	equipmentRepo := &fakeEquipmentRepo{byID: map[uuid.UUID]domain.EquipmentItem{equipment.ID(): equipment}}
	userRepo := &fakeUserRepo{byID: map[uuid.UUID]domain.User{knownUser.ID(): knownUser}}
	publisher := &fakePublisher{}

	service := app.NewExposureService(exposureRepo, equipmentRepo, userRepo, publisher)
	router := NewRouter(NewExposureHandler(service), NewExposureSummaryHandler(service))

	req := httptest.NewRequest("GET", "/exposure", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != 200 {
		t.Fatalf("GET /exposure: status = %d, body = %s", rec.Code, rec.Body.String())
	}

	if publisher.publishOrphanedCalls != 1 {
		t.Errorf("PublishOrphaned calls = %d, want 1 (should batch both orphans into one call)", publisher.publishOrphanedCalls)
	}
	if len(publisher.publishedOrphaned) != 2 {
		t.Fatalf("expected 2 orphaned events published, got %d", len(publisher.publishedOrphaned))
	}
	gotUserIDs := map[uuid.UUID]bool{
		publisher.publishedOrphaned[0].UserID: true,
		publisher.publishedOrphaned[1].UserID: true,
	}
	if !gotUserIDs[orphanedUserA] || !gotUserIDs[orphanedUserB] {
		t.Errorf("published orphaned events = %v, want both %v and %v", publisher.publishedOrphaned, orphanedUserA, orphanedUserB)
	}
}

// TestListExposures_SurvivesOrphanedEventPublishFailure asserts that a
// failure publishing the orphaned-exposure event does not itself break the
// list request — the publish is best-effort. Without this, reporting the
// orphan via Redis could reintroduce the exact "one bad record 500s the
// whole list" failure this code path exists to prevent.
func TestListExposures_SurvivesOrphanedEventPublishFailure(t *testing.T) {
	equipment, err := domain.NewEquipmentItem(uuid.MustParse("2e85d43d-dd9b-4e8d-b2ce-97b8d7d69d49"), "AirCat - Drill - 4337", 2.1)
	if err != nil {
		t.Fatalf("NewEquipmentItem: %v", err)
	}
	knownUser, err := domain.NewUser(uuid.New(), "Known User")
	if err != nil {
		t.Fatalf("NewUser: %v", err)
	}
	orphanedUserID := uuid.New()
	duration, err := domain.NewMinutes(5)
	if err != nil {
		t.Fatalf("NewMinutes: %v", err)
	}

	exposureRepo := newFakeExposureRepo()
	if _, err := exposureRepo.Create(context.Background(), mustExposure(t, uuid.New(), equipment, knownUser.ID(), duration, time.Now())); err != nil {
		t.Fatalf("seed knownUser exposure: %v", err)
	}
	if _, err := exposureRepo.Create(context.Background(), mustExposure(t, uuid.New(), equipment, orphanedUserID, duration, time.Now())); err != nil {
		t.Fatalf("seed orphaned exposure: %v", err)
	}

	equipmentRepo := &fakeEquipmentRepo{byID: map[uuid.UUID]domain.EquipmentItem{equipment.ID(): equipment}}
	userRepo := &fakeUserRepo{byID: map[uuid.UUID]domain.User{knownUser.ID(): knownUser}}
	publisher := &fakePublisher{orphanedErr: errors.New("redis unavailable")}

	service := app.NewExposureService(exposureRepo, equipmentRepo, userRepo, publisher)
	router := NewRouter(NewExposureHandler(service), NewExposureSummaryHandler(service))

	req := httptest.NewRequest("GET", "/exposure", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != 200 {
		t.Fatalf("GET /exposure: status = %d, body = %s (a failed orphan-event publish should not fail the list)", rec.Code, rec.Body.String())
	}

	var list []exposureResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &list); err != nil {
		t.Fatalf("decode list: %v", err)
	}
	if len(list) != 1 {
		t.Fatalf("expected 1 resolvable exposure in list (orphan excluded), got %d", len(list))
	}
}

// TestGetExposure_ReportsOrphanedUser asserts that GET /exposure/{id} on an
// orphaned exposure (existing exposure, unresolvable user) returns 500, not
// 404 — the exposure genuinely exists, so this is not a client-side error.
// It should also publish an ExposureOrphanedEvent, matching the list path's behavior.
func TestGetExposure_ReportsOrphanedUser(t *testing.T) {
	equipment, err := domain.NewEquipmentItem(uuid.MustParse("2e85d43d-dd9b-4e8d-b2ce-97b8d7d69d49"), "AirCat - Drill - 4337", 2.1)
	if err != nil {
		t.Fatalf("NewEquipmentItem: %v", err)
	}
	orphanedUserID := uuid.New()
	duration, err := domain.NewMinutes(5)
	if err != nil {
		t.Fatalf("NewMinutes: %v", err)
	}

	exposureRepo := newFakeExposureRepo()
	exposure := mustExposure(t, uuid.New(), equipment, orphanedUserID, duration, time.Now())
	if _, err := exposureRepo.Create(context.Background(), exposure); err != nil {
		t.Fatalf("seed orphaned exposure: %v", err)
	}

	equipmentRepo := &fakeEquipmentRepo{byID: map[uuid.UUID]domain.EquipmentItem{equipment.ID(): equipment}}
	userRepo := &fakeUserRepo{byID: map[uuid.UUID]domain.User{}} // orphanedUserID deliberately absent
	publisher := &fakePublisher{}

	service := app.NewExposureService(exposureRepo, equipmentRepo, userRepo, publisher)
	router := NewRouter(NewExposureHandler(service), NewExposureSummaryHandler(service))

	req := httptest.NewRequest("GET", "/exposure/"+exposure.ID().String(), nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != 500 {
		t.Fatalf("GET /exposure/{id} orphaned: status = %d, want 500 (exposure exists; only the user reference is broken)", rec.Code)
	}

	if len(publisher.publishedOrphaned) != 1 {
		t.Fatalf("expected 1 published orphaned-exposure event, got %d", len(publisher.publishedOrphaned))
	}
	if publisher.publishedOrphaned[0].ExposureID != exposure.ID() {
		t.Errorf("published event ExposureID = %v, want %v", publisher.publishedOrphaned[0].ExposureID, exposure.ID())
	}
}
