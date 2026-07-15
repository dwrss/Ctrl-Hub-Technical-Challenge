package http

import (
	"context"
	"encoding/json"
	"math"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/google/uuid"

	"equipment-exposure-service/internal/app"
	"equipment-exposure-service/internal/domain"
)

func TestExposureSummaryEndpoint(t *testing.T) {
	equipment, err := domain.NewEquipmentItem(uuid.MustParse("2e85d43d-dd9b-4e8d-b2ce-97b8d7d69d49"), "AirCat - Drill - 4337", 2.1)
	if err != nil {
		t.Fatalf("NewEquipmentItem: %v", err)
	}
	user, err := domain.NewUser(uuid.MustParse("713be58e-0d79-4df2-a85c-9f44ca513a7d"), "Bobby Tables")
	if err != nil {
		t.Fatalf("NewUser: %v", err)
	}
	duration, err := domain.NewMinutes(5)
	if err != nil {
		t.Fatalf("NewMinutes: %v", err)
	}

	exposureRepo := newFakeExposureRepo()
	exposure := mustExposure(t, uuid.New(), equipment, user.ID(), duration, time.Now())
	if _, err := exposureRepo.Create(context.Background(), exposure); err != nil {
		t.Fatalf("seed exposure: %v", err)
	}
	equipmentRepo := &fakeEquipmentRepo{byID: map[uuid.UUID]domain.EquipmentItem{equipment.ID(): equipment}}
	userRepo := &fakeUserRepo{byID: map[uuid.UUID]domain.User{user.ID(): user}}
	publisher := &fakePublisher{}

	service := app.NewExposureService(exposureRepo, equipmentRepo, userRepo, publisher)
	router := NewRouter(NewExposureHandler(service), NewExposureSummaryHandler(service))

	// GET /users/{userId}/exposure-summary should reflect the recorded exposure.
	req := httptest.NewRequest("GET", "/users/"+user.ID().String()+"/exposure-summary", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != 200 {
		t.Fatalf("GET exposure-summary: status = %d, body = %s", rec.Code, rec.Body.String())
	}
	var summary exposureSummaryResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &summary); err != nil {
		t.Fatalf("decode summary: %v", err)
	}
	if summary.A8 != exposure.A8() {
		t.Errorf("summary.a8 = %v, want %v", summary.A8, exposure.A8())
	}
	if summary.Points != exposure.Points() {
		t.Errorf("summary.points = %v, want %v", summary.Points, exposure.Points())
	}
	if summary.User.Name != "Bobby Tables" {
		t.Errorf("summary.user.name = %q, want %q", summary.User.Name, "Bobby Tables")
	}

	// GET exposure-summary for an unknown user should 404.
	req = httptest.NewRequest("GET", "/users/"+uuid.New().String()+"/exposure-summary", nil)
	rec = httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != 404 {
		t.Fatalf("GET exposure-summary unknown user: status = %d, want 404", rec.Code)
	}
}

// TestExposureSummaryWindowing exercises what TestExposureSummaryEndpoint
// doesn't: starting_at/ending_at parsing and filtering, multi-exposure
// aggregation, and malformed query params. A single-exposure, no-window
// summary can't catch a broken date filter or a broken aggregation (e.g.
// RSS vs. linear sum flipped), since with one exposure both would
// coincidentally agree.
func TestExposureSummaryWindowing(t *testing.T) {
	equipment, err := domain.NewEquipmentItem(uuid.MustParse("2e85d43d-dd9b-4e8d-b2ce-97b8d7d69d49"), "AirCat - Drill - 4337", 2.1)
	if err != nil {
		t.Fatalf("NewEquipmentItem: %v", err)
	}
	jcbBreaker, err := domain.NewEquipmentItem(uuid.MustParse("36603447-2f30-41b1-a908-526c0b6f1755"), "JCB - Hydraulic Breaker - CEJCBHM25", 4.0)
	if err != nil {
		t.Fatalf("NewEquipmentItem: %v", err)
	}
	user, err := domain.NewUser(uuid.MustParse("713be58e-0d79-4df2-a85c-9f44ca513a7d"), "Bobby Tables")
	if err != nil {
		t.Fatalf("NewUser: %v", err)
	}

	jan1 := time.Date(2025, 1, 1, 12, 0, 0, 0, time.UTC)
	jan15 := time.Date(2025, 1, 15, 12, 0, 0, 0, time.UTC)
	feb1 := time.Date(2025, 2, 1, 12, 0, 0, 0, time.UTC)

	inWindowDuration, err := domain.NewMinutes(120)
	if err != nil {
		t.Fatalf("NewMinutes: %v", err)
	}
	outOfWindowDuration, err := domain.NewMinutes(300)
	if err != nil {
		t.Fatalf("NewMinutes: %v", err)
	}

	inWindow := mustExposure(t, uuid.New(), equipment, user.ID(), inWindowDuration, jan15)
	outOfWindow := mustExposure(t, uuid.New(), jcbBreaker, user.ID(), outOfWindowDuration, feb1)

	exposureRepo := newFakeExposureRepo()
	if _, err := exposureRepo.Create(context.Background(), inWindow); err != nil {
		t.Fatalf("seed inWindow: %v", err)
	}
	if _, err := exposureRepo.Create(context.Background(), outOfWindow); err != nil {
		t.Fatalf("seed outOfWindow: %v", err)
	}
	equipmentRepo := &fakeEquipmentRepo{byID: map[uuid.UUID]domain.EquipmentItem{
		equipment.ID():  equipment,
		jcbBreaker.ID(): jcbBreaker,
	}}
	userRepo := &fakeUserRepo{byID: map[uuid.UUID]domain.User{user.ID(): user}}
	publisher := &fakePublisher{}

	service := app.NewExposureService(exposureRepo, equipmentRepo, userRepo, publisher)
	router := NewRouter(NewExposureHandler(service), NewExposureSummaryHandler(service))

	// A window covering only inWindow (Jan 1 - Jan 31) should exclude
	// outOfWindow (Feb 1) and match inWindow's own A8/Points exactly.
	url := "/users/" + user.ID().String() + "/exposure-summary" +
		"?starting_at=" + jan1.Format(time.RFC3339) +
		"&ending_at=" + time.Date(2025, 1, 31, 23, 59, 59, 0, time.UTC).Format(time.RFC3339)
	req := httptest.NewRequest("GET", url, nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != 200 {
		t.Fatalf("GET exposure-summary windowed: status = %d, body = %s", rec.Code, rec.Body.String())
	}
	var summary exposureSummaryResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &summary); err != nil {
		t.Fatalf("decode summary: %v", err)
	}
	if summary.A8 != inWindow.A8() {
		t.Errorf("windowed summary.a8 = %v, want %v (outOfWindow should be excluded)", summary.A8, inWindow.A8())
	}
	if summary.Points != inWindow.Points() {
		t.Errorf("windowed summary.points = %v, want %v (outOfWindow should be excluded)", summary.Points, inWindow.Points())
	}

	// No window at all should aggregate both exposures: points sums
	// linearly, a8 combines via root-sum-of-squares (not a linear sum).
	req = httptest.NewRequest("GET", "/users/"+user.ID().String()+"/exposure-summary", nil)
	rec = httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != 200 {
		t.Fatalf("GET exposure-summary unwindowed: status = %d, body = %s", rec.Code, rec.Body.String())
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &summary); err != nil {
		t.Fatalf("decode summary: %v", err)
	}
	wantPoints := inWindow.Points() + outOfWindow.Points()
	if summary.Points != wantPoints {
		t.Errorf("unwindowed summary.points = %v, want linear sum %v", summary.Points, wantPoints)
	}
	wantA8 := math.Sqrt(inWindow.A8()*inWindow.A8() + outOfWindow.A8()*outOfWindow.A8())
	if math.Abs(summary.A8-wantA8) > 1e-9 {
		t.Errorf("unwindowed summary.a8 = %v, want RSS %v", summary.A8, wantA8)
	}
	linearA8 := inWindow.A8() + outOfWindow.A8()
	if math.Abs(summary.A8-linearA8) < 1e-9 {
		t.Errorf("unwindowed summary.a8 = %v unexpectedly matches naive linear sum %v", summary.A8, linearA8)
	}

	// A window before either exposure should report a zero-value summary,
	// not an error.
	req = httptest.NewRequest("GET", "/users/"+user.ID().String()+"/exposure-summary?ending_at="+jan1.Format(time.RFC3339), nil)
	rec = httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != 200 {
		t.Fatalf("GET exposure-summary empty window: status = %d, body = %s", rec.Code, rec.Body.String())
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &summary); err != nil {
		t.Fatalf("decode summary: %v", err)
	}
	if summary.A8 != 0 || summary.Points != 0 {
		t.Errorf("empty window summary = {a8: %v, points: %v}, want zero value", summary.A8, summary.Points)
	}

	// A date-only starting_at/ending_at (spec's declared "format: date",
	// as opposed to its RFC3339 examples) should be accepted and treated
	// as midnight UTC on that date.
	req = httptest.NewRequest("GET", "/users/"+user.ID().String()+"/exposure-summary?starting_at=2025-01-01&ending_at=2025-01-31", nil)
	rec = httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != 200 {
		t.Fatalf("GET exposure-summary date-only window: status = %d, body = %s", rec.Code, rec.Body.String())
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &summary); err != nil {
		t.Fatalf("decode summary: %v", err)
	}
	if summary.A8 != inWindow.A8() {
		t.Errorf("date-only windowed summary.a8 = %v, want %v (outOfWindow should be excluded)", summary.A8, inWindow.A8())
	}
	if summary.Points != inWindow.Points() {
		t.Errorf("date-only windowed summary.points = %v, want %v (outOfWindow should be excluded)", summary.Points, inWindow.Points())
	}

	// Malformed starting_at/ending_at should 400, not 500 or a silently
	// ignored filter.
	for _, tt := range []struct {
		name  string
		query string
	}{
		{"bad starting_at", "?starting_at=not-a-date"},
		{"bad ending_at", "?ending_at=not-a-date"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest("GET", "/users/"+user.ID().String()+"/exposure-summary"+tt.query, nil)
			rec := httptest.NewRecorder()
			router.ServeHTTP(rec, req)
			if rec.Code != 400 {
				t.Errorf("status = %d, want 400", rec.Code)
			}
		})
	}
}
