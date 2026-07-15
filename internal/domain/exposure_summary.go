package domain

import "math"

type ExposureSummary struct {
	user   User
	a8     float64
	points float64
}

func (s ExposureSummary) User() User      { return s.user }
func (s ExposureSummary) A8() float64     { return s.a8 }
func (s ExposureSummary) Points() float64 { return s.points }

// ExposureAccumulator holds the two additive running totals needed to
// finalise an ExposureSummary:
// - points (additive)
// - the sum of squared A8
// It really exists so we can build a clean-ish mock of the exposure repository.
type ExposureAccumulator struct {
	PointsTotal    float64
	A8SquaredTotal float64
}

// Add folds a single exposure's contribution into the accumulator.
func (a ExposureAccumulator) Add(e Exposure) ExposureAccumulator {
	a8 := e.A8()
	return ExposureAccumulator{
		PointsTotal:    a.PointsTotal + e.Points(),
		A8SquaredTotal: a.A8SquaredTotal + a8*a8,
	}
}

// FinalizeExposureSummary turns an accumulator into an ExposureSummary.
//
// Points is summed linearly: it is designed as an additive percentage-of-
// daily-limit scale (100 points == one full day's exposure at the action
// level), so summing partial contributions is correct.
//
// A8 is combined via root-sum-of-squares (quadrature), matching how HSE
// combines partial A(8) contributions from multiple tools/sessions in a day.
// A8 is not summed linearly:
// Points_i == 16 * A8_i^2 holds exactly ((M/2.5)^2 * f * 100 = M^2 / 6.25) * f * 100 = 16 * M^2 * f = 16 * A8^2)
// for every exposure (verified in exposure_test.go), a linear
// sum of A8 would break that relationship at the aggregate level. Using
// A8_total = sqrt(sum(A8_i^2)) preserves that relationship.
func FinalizeExposureSummary(user User, acc ExposureAccumulator) ExposureSummary {
	return ExposureSummary{
		user:   user,
		a8:     math.Sqrt(acc.A8SquaredTotal),
		points: acc.PointsTotal,
	}
}
