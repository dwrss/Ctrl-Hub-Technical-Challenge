package mongodb

import (
	"testing"
	"time"

	"github.com/google/uuid"

	"equipment-exposure-service/internal/domain"
)

// TestToExposureDoc_StoresA8AndPoints guards against a8/points silently
// becoming write-only: RecordExposure computes them once via the domain
// entity, and toExposureDoc must actually persist those values rather than
// dropping them (e.g. a missing bson tag).
func TestToExposureDoc_StoresA8AndPoints(t *testing.T) {
	equipment, err := domain.NewEquipmentItem(uuid.New(), "AirCat - Drill - 4337", 2.1)
	if err != nil {
		t.Fatalf("NewEquipmentItem: %v", err)
	}
	duration, err := domain.NewMinutes(5)
	if err != nil {
		t.Fatalf("NewMinutes: %v", err)
	}
	exposure, err := domain.NewExposure(uuid.New(), equipment, uuid.New(), duration, time.Now().UTC())
	if err != nil {
		t.Fatalf("NewExposure: %v", err)
	}

	doc := toExposureDoc(exposure)
	if doc.A8 != exposure.A8() {
		t.Errorf("doc.A8 = %v, want %v", doc.A8, exposure.A8())
	}
	if doc.Points != exposure.Points() {
		t.Errorf("doc.Points = %v, want %v", doc.Points, exposure.Points())
	}

	reconstructed, err := fromExposureDoc(doc)
	if err != nil {
		t.Fatalf("fromExposureDoc: %v", err)
	}
	if reconstructed.A8() != exposure.A8() {
		t.Errorf("reconstructed.A8() = %v, want %v", reconstructed.A8(), exposure.A8())
	}
	if reconstructed.Points() != exposure.Points() {
		t.Errorf("reconstructed.Points() = %v, want %v", reconstructed.Points(), exposure.Points())
	}
}
