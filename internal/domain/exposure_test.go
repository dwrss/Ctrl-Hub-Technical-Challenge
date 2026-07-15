package domain

import (
	"math"
	"testing"
	"time"

	"github.com/google/uuid"
)

// Helpers

func mustEquipment(t *testing.T, name string, vibrationMagnitude float64) EquipmentItem {
	t.Helper()
	e, err := NewEquipmentItem(uuid.New(), name, vibrationMagnitude)
	if err != nil {
		t.Fatalf("NewEquipmentItem: %v", err)
	}
	return e
}

func mustMinutes(t *testing.T, value int) Minutes {
	t.Helper()
	m, err := NewMinutes(value)
	if err != nil {
		t.Fatalf("NewMinutes: %v", err)
	}
	return m
}

func mustExposure(t *testing.T, id uuid.UUID, equipment EquipmentItem, userID uuid.UUID, duration Minutes, occurredAt time.Time) Exposure {
	t.Helper()
	e, err := NewExposure(id, equipment, userID, duration, occurredAt)
	if err != nil {
		t.Fatalf("NewExposure: %v", err)
	}
	return e
}

func testFixtures(t *testing.T) (airCatDrill, jcbBreaker EquipmentItem, user User) {
	t.Helper()
	airCatDrill = mustEquipment(t, "AirCat - Drill - 4337", 2.1)
	jcbBreaker = mustEquipment(t, "JCB - Hydraulic Breaker - CEJCBHM25", 4.0)
	u, err := NewUser(uuid.New(), "Bobby Tables")
	if err != nil {
		t.Fatalf("NewUser: %v", err)
	}
	return airCatDrill, jcbBreaker, u
}

// Tests

func TestExposure_A8(t *testing.T) {
	airCatDrill, jcbBreaker, user := testFixtures(t)

	tests := []struct {
		name      string
		equipment EquipmentItem
		duration  int
		want      float64
	}{
		// At exactly 8 hours (480 min) of use, dailyFraction == 1, so
		// A8 should equal the equipment's vibration magnitude exactly.
		{"airCatDrill full 8h shift", airCatDrill, 480, 2.1},
		{"jcbBreaker full 8h shift", jcbBreaker, 480, 4.0},
		// spec.yaml's own example duration (5 minutes).
		{"airCatDrill 5 minutes", airCatDrill, 5, 2.1 * math.Sqrt((5.0/60)/8)},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			e := mustExposure(t, uuid.New(), tt.equipment, user.ID(), mustMinutes(t, tt.duration), time.Now())
			got := e.A8()
			if math.Abs(got-tt.want) > 1e-9 {
				t.Errorf("A8() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestExposure_Points(t *testing.T) {
	airCatDrill, jcbBreaker, user := testFixtures(t)

	tests := []struct {
		name      string
		equipment EquipmentItem
		duration  int
		want      float64
	}{
		{"airCatDrill full 8h shift", airCatDrill, 480, math.Round(math.Pow(2.1/2.5, 2) * 100)},
		{"jcbBreaker full 8h shift", jcbBreaker, 480, math.Round(math.Pow(4.0/2.5, 2) * 100)},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			e := mustExposure(t, uuid.New(), tt.equipment, user.ID(), mustMinutes(t, tt.duration), time.Now())
			got := e.Points()
			if got != tt.want {
				t.Errorf("Points() = %v, want %v", got, tt.want)
			}
		})
	}
}

// TestExposure_PointsA8Invariant confirms Points == 16 * A8^2 holds for a
// single exposure across a range of equipment/durations. This is the
// identity that SummarizeExposures relies on to justify combining A8 via
// root-sum-of-squares rather than a linear sum.
func TestExposure_PointsA8Invariant(t *testing.T) {
	airCatDrill, jcbBreaker, user := testFixtures(t)
	durations := []int{1, 5, 60, 245, 480}
	equipment := []EquipmentItem{airCatDrill, jcbBreaker}

	for _, eq := range equipment {
		for _, d := range durations {
			e := mustExposure(t, uuid.New(), eq, user.ID(), mustMinutes(t, d), time.Now())
			a8 := e.A8()
			points := e.Points()
			want := math.Round(16 * a8 * a8)
			if points != want {
				t.Errorf("equipment=%s duration=%d: Points()=%v, want 16*A8^2=%v", eq.Name(), d, points, want)
			}
		}
	}
}

func TestNewMinutes_RejectsNonPositive(t *testing.T) {
	for _, v := range []int{0, -1, -100} {
		if _, err := NewMinutes(v); err == nil {
			t.Errorf("NewMinutes(%d) = nil error, want error", v)
		}
	}
}

func TestNewEquipmentItem_RejectsNonPositiveVibrationMagnitude(t *testing.T) {
	for _, v := range []float64{0, -1} {
		if _, err := NewEquipmentItem(uuid.New(), "bad equipment", v); err == nil {
			t.Errorf("NewEquipmentItem(vibrationMagnitude=%v) = nil error, want error", v)
		}
	}
}

// TestNewExposure_RejectsInvalidInput proves NewExposure itself validates
// its inputs, rather than relying entirely on callers to pre-validate
// through NewMinutes/NewEquipmentItem. Both bypasses below are reachable
// from outside the domain package: Minutes is a plain exported int
// (domain.Minutes(0) compiles anywhere), and EquipmentItem{} is a
// constructible zero value despite its unexported fields.
func TestNewExposure_RejectsInvalidInput(t *testing.T) {
	validEquipment := mustEquipment(t, "AirCat - Drill - 4337", 2.1)
	validDuration := mustMinutes(t, 5)

	if _, err := NewExposure(uuid.New(), validEquipment, uuid.New(), Minutes(0), time.Now()); err == nil {
		t.Error("NewExposure with Minutes(0) = nil error, want error")
	}
	if _, err := NewExposure(uuid.New(), validEquipment, uuid.New(), Minutes(-5), time.Now()); err == nil {
		t.Error("NewExposure with negative Minutes = nil error, want error")
	}
	if _, err := NewExposure(uuid.New(), EquipmentItem{}, uuid.New(), validDuration, time.Now()); err == nil {
		t.Error("NewExposure with zero-value EquipmentItem = nil error, want error")
	}
}
