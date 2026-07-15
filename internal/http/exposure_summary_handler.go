package http

import (
	"errors"
	"log"
	"net/http"
	"time"

	"github.com/google/uuid"

	"equipment-exposure-service/internal/app"
	"equipment-exposure-service/internal/domain"
)

type ExposureSummaryHandler struct {
	service *app.ExposureService
}

func NewExposureSummaryHandler(service *app.ExposureService) *ExposureSummaryHandler {
	return &ExposureSummaryHandler{service: service}
}

// Get handles GET /users/{userId}/exposure-summary.
// Note: spec.yaml declares `format: date` for `starting_at`/`ending_at`,
// but its own examples are full RFC3339 timestamps, so parseOptionalTime accepts both.
func (h *ExposureSummaryHandler) Get(w http.ResponseWriter, r *http.Request) {
	userID, err := uuid.Parse(r.PathValue("userId"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid userId")
		return
	}

	from, err := parseOptionalTime(r.URL.Query().Get("starting_at"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid starting_at")
		return
	}

	to, err := parseOptionalTime(r.URL.Query().Get("ending_at"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid ending_at")
		return
	}

	summary, err := h.service.GetUserExposureSummary(r.Context(), userID, from, to)
	log.Printf("GetUserExposureSummary(%s, %s, %s) -> %v, %v", userID, from, to, summary, err)
	if errors.Is(err, domain.ErrNotFound) {
		writeError(w, http.StatusNotFound, "user not found")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, toExposureSummaryResponse(summary))
}

// parseOptionalTime accepts either a date-only value (spec's declared "format: date")
// or a full RFC3339 timestamp (used in the spec's own examples). A date-only value
// is treated as midnight UTC.
func parseOptionalTime(value string) (*time.Time, error) {
	if value == "" {
		return nil, nil
	}
	if t, err := time.Parse(time.RFC3339, value); err == nil {
		return &t, nil
	}
	t, err := time.Parse(time.DateOnly, value)
	if err != nil {
		return nil, err
	}
	return &t, nil
}
