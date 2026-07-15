package domain

import "fmt"

// Minutes is a positive duration of equipment use. It exists so "duration"
// can't silently be zero, negative, or unit-confused anywhere it's passed
// around — the only way to get a Minutes value is through NewMinutes.
type Minutes int

func NewMinutes(value int) (Minutes, error) {
	if value <= 0 {
		return 0, fmt.Errorf("%w: duration must be positive, got %d", ErrInvalidInput, value)
	}
	return Minutes(value), nil
}

func (m Minutes) Int() int {
	return int(m)
}

func (m Minutes) dailyFraction() float64 {
	return (float64(m) / 60) / 8
}
