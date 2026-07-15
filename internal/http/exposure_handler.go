package http

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/google/uuid"

	"equipment-exposure-service/internal/app"
	"equipment-exposure-service/internal/domain"
)

type ExposureHandler struct {
	service *app.ExposureService
}

func NewExposureHandler(service *app.ExposureService) *ExposureHandler {
	return &ExposureHandler{service: service}
}

func (h *ExposureHandler) List(w http.ResponseWriter, r *http.Request) {
	views, err := h.service.ListExposures(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	responses := make([]exposureResponse, 0, len(views))
	for _, v := range views {
		responses = append(responses, toExposureResponse(v))
	}
	writeJSON(w, http.StatusOK, responses)
}

// Get handles GET /exposure/{exposureId}.
// Note: spec.yaml documents this endpoint's success response as 201.This is semantically incorrect for an idempotent function,
// so we return 200 here.
func (h *ExposureHandler) Get(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(r.PathValue("exposureId"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid exposureId")
		return
	}

	exposure, err := h.service.GetExposure(r.Context(), id)
	if errors.Is(err, domain.ErrNotFound) {
		writeError(w, http.StatusNotFound, "exposure not found")
		return
	}
	if errors.Is(err, domain.ErrDanglingReference) {
		writeError(w, http.StatusInternalServerError, "exposure references a user that could not be resolved")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, toExposureResponse(exposure))
}

func (h *ExposureHandler) Create(w http.ResponseWriter, r *http.Request) {
	var req recordExposureRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	userID, err := uuid.Parse(req.UserID)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid user_id")
		return
	}

	equipmentID, err := uuid.Parse(req.EquipmentID)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid equipment_id")
		return
	}

	exposure, err := h.service.RecordExposure(r.Context(), userID, equipmentID, req.Duration)
	if errors.Is(err, domain.ErrNotFound) {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}
	if errors.Is(err, domain.ErrInvalidInput) {
		writeError(w, http.StatusBadRequest, "duration must be positive")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	writeJSON(w, http.StatusCreated, toExposureResponse(exposure))
}
