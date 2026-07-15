package http

import (
	"equipment-exposure-service/internal/app"
	"equipment-exposure-service/internal/domain"
)

type userResponse struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

type equipmentResponse struct {
	ID                 string  `json:"id"`
	Name               string  `json:"name"`
	VibrationMagnitude float64 `json:"vibration_magnitude"`
}

type exposureResponse struct {
	ID        string            `json:"id"`
	Equipment equipmentResponse `json:"equipment"`
	Duration  int               `json:"duration"`
	A8        float64           `json:"a8"`
	Points    float64           `json:"points"`
	User      userResponse      `json:"user"`
}

type exposureSummaryResponse struct {
	A8     float64      `json:"a8"`
	Points float64      `json:"points"`
	User   userResponse `json:"user"`
}

type recordExposureRequest struct {
	EquipmentID string `json:"equipment_id"`
	Duration    int    `json:"duration"`
	UserID      string `json:"user_id"`
}

func toUserResponse(u domain.User) userResponse {
	return userResponse{ID: u.ID().String(), Name: u.Name()}
}

func toEquipmentResponse(e domain.EquipmentItem) equipmentResponse {
	return equipmentResponse{
		ID:                 e.ID().String(),
		Name:               e.Name(),
		VibrationMagnitude: e.VibrationMagnitude(),
	}
}

func toExposureResponse(v app.ExposureView) exposureResponse {
	e := v.Exposure
	return exposureResponse{
		ID:        e.ID().String(),
		Equipment: toEquipmentResponse(e.Equipment()),
		Duration:  e.Duration().Int(),
		A8:        e.A8(),
		Points:    e.Points(),
		User:      toUserResponse(v.User),
	}
}

func toExposureSummaryResponse(s domain.ExposureSummary) exposureSummaryResponse {
	return exposureSummaryResponse{
		A8:     s.A8(),
		Points: s.Points(),
		User:   toUserResponse(s.User()),
	}
}
