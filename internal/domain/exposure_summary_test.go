package domain

import (
	"math"
	"testing"
	"time"

	"github.com/google/uuid"
)

func TestSummarizeExposures(t *testing.T) {
	airCatDrill, jcbBreaker, user := testFixtures(t)
	exposures := []Exposure{
		mustExposure(t, uuid.New(), airCatDrill, user.ID(), mustMinutes(t, 120), time.Now()),
		mustExposure(t, uuid.New(), jcbBreaker, user.ID(), mustMinutes(t, 45), time.Now()),
		mustExposure(t, uuid.New(), airCatDrill, user.ID(), mustMinutes(t, 200), time.Now()),
	}

	summary := SummarizeExposures(user, exposures)

	// points must equal the linear sum of each exposure's points.
	var wantPoints float64
	for _, e := range exposures {
		wantPoints += e.Points()
	}
	if summary.Points() != wantPoints {
		t.Errorf("Points = %v, want linear sum %v", summary.Points(), wantPoints)
	}

	// a8 must be the root-sum-of-squares, not the linear sum.
	var sumOfSquares, linearSum float64
	for _, e := range exposures {
		a8 := e.A8()
		sumOfSquares += a8 * a8
		linearSum += a8
	}
	wantA8 := math.Sqrt(sumOfSquares)
	if math.Abs(summary.A8()-wantA8) > 1e-9 {
		t.Errorf("A8 = %v, want root-sum-of-squares %v", summary.A8(), wantA8)
	}
	if math.Abs(summary.A8()-linearSum) < 1e-9 {
		t.Errorf("A8 = %v unexpectedly matches naive linear sum %v; root-sum-of-squares should differ here", summary.A8(), linearSum)
	}

	// the invariant Points == 16 * A8^2 should still hold at the aggregate
	// level, within rounding error introduced by rounding each exposure's
	// points individually.
	if math.Abs(summary.Points()-16*summary.A8()*summary.A8()) > float64(len(exposures)) {
		t.Errorf("aggregate invariant broken: Points=%v, 16*A8^2=%v", summary.Points(), 16*summary.A8()*summary.A8())
	}
}
