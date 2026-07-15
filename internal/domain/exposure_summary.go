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

// SummarizeExposures aggregates a user's exposures into a single summary.
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
// A8_total = sqrt(sum(A8_i^2)) preserves Points_total == 16 * A8_total^2.
func SummarizeExposures(user User, exposures []Exposure) ExposureSummary {
	var pointsTotal float64
	var a8SquaredTotal float64

	for _, e := range exposures {
		pointsTotal += e.Points()
		a8 := e.A8()
		a8SquaredTotal += a8 * a8
	}

	return ExposureSummary{
		user:   user,
		a8:     math.Sqrt(a8SquaredTotal),
		points: pointsTotal,
	}
}
