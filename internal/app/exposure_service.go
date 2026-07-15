package app

import (
	"context"
	"errors"
	"fmt"
	"log"
	"time"

	"github.com/google/uuid"

	"equipment-exposure-service/internal/domain"
)

type ExposureService struct {
	exposures domain.ExposureRepository
	equipment domain.EquipmentRepository
	users     domain.UserRepository
	publisher EventPublisher
}

func NewExposureService(
	exposures domain.ExposureRepository,
	equipment domain.EquipmentRepository,
	users domain.UserRepository,
	publisher EventPublisher,
) *ExposureService {
	return &ExposureService{
		exposures: exposures,
		equipment: equipment,
		users:     users,
		publisher: publisher,
	}
}

// ExposureView pairs an Exposure with its resolved User, since Exposure
// itself only carries a UserID. This is what read/create paths return so
// the HTTP layer can render the spec's nested "user" field without
// reaching into a repository itself.
type ExposureView struct {
	Exposure domain.Exposure
	User     domain.User
}

func (s *ExposureService) ListExposures(ctx context.Context) ([]ExposureView, error) {
	exposures, err := s.exposures.List(ctx)
	if err != nil {
		return nil, err
	}
	return s.resolveViews(ctx, exposures)
}

func (s *ExposureService) GetExposure(ctx context.Context, id uuid.UUID) (ExposureView, error) {
	exposure, err := s.exposures.Get(ctx, id)
	if err != nil {
		return ExposureView{}, err
	}
	user, err := s.users.Get(ctx, exposure.UserID())
	if errors.Is(err, domain.ErrNotFound) {
		// We have an exposure without an associated user. This should never happen and represents a data-integrity issue.
		s.reportOrphanedExposures(ctx, []domain.Exposure{exposure})
		return ExposureView{}, domain.ErrDanglingReference
	}
	if err != nil {
		return ExposureView{}, err
	}
	return ExposureView{Exposure: exposure, User: user}, nil
}

// RecordExposure looks up the referenced user and equipment, computes the
// A8/Points for the new exposure via the domain entity, persists it, and
// publishes an ExposureRecordedEvent on success.
func (s *ExposureService) RecordExposure(ctx context.Context, userID, equipmentID uuid.UUID, durationMinutes int) (ExposureView, error) {
	duration, err := domain.NewMinutes(durationMinutes)
	if err != nil {
		return ExposureView{}, err
	}

	user, err := s.users.Get(ctx, userID)
	if err != nil {
		return ExposureView{}, fmt.Errorf("failed to get user: %w", err)
	}

	equipment, err := s.equipment.Get(ctx, equipmentID)
	if err != nil {
		return ExposureView{}, fmt.Errorf("failed to get equipment: %w", err)
	}

	exposure, err := domain.NewExposure(uuid.New(), equipment, user.ID(), duration, time.Now().UTC())
	if err != nil {
		return ExposureView{}, fmt.Errorf("failed to record exposure: %w", err)
	}

	created, err := s.exposures.Create(ctx, exposure)
	if err != nil {
		return ExposureView{}, err
	}

	event := ExposureRecordedEvent{
		ExposureID:  created.ID(),
		UserID:      created.UserID(),
		EquipmentID: created.Equipment().ID(),
		A8:          created.A8(),
		Points:      created.Points(),
		OccurredAt:  created.OccurredAt(),
	}
	if err := s.publisher.Publish(ctx, event); err != nil {
		return ExposureView{}, err
	}

	return ExposureView{Exposure: created, User: user}, nil
}

// resolveViews resolves the User associated with each exposure via a batched lookup.
//
// An exposure whose user can no longer be resolved indicates a data-integrity issue.
// If such an exposure is present, it is omitted from the result and an ExposureOrphanedEvent published.

func (s *ExposureService) resolveViews(ctx context.Context, exposures []domain.Exposure) ([]ExposureView, error) {
	idsSeen := make(map[uuid.UUID]struct{}, len(exposures))
	idsToLookup := make([]uuid.UUID, 0, len(exposures))
	for _, e := range exposures {
		if _, ok := idsSeen[e.UserID()]; !ok {
			idsSeen[e.UserID()] = struct{}{}
			idsToLookup = append(idsToLookup, e.UserID())
		}
	}

	users, err := s.users.GetMany(ctx, idsToLookup)
	if err != nil {
		return nil, err
	}

	views := make([]ExposureView, 0, len(exposures))
	var orphaned []domain.Exposure
	for _, e := range exposures {
		user, ok := users[e.UserID()]
		if !ok {
			orphaned = append(orphaned, e)
			continue
		}
		views = append(views, ExposureView{Exposure: e, User: user})
	}
	s.reportOrphanedExposures(ctx, orphaned)

	return views, nil
}

// reportOrphanedExposures logs each exposure whose user can't be resolved,
// then publishes them as a single batch of ExposureOrphanedEvents in one
// round trip (which vastly improves Redis's throughput).
// The publish is best-effort: its error is only logged, never returned, as we don't want to propagate internal errors
// to the caller.
func (s *ExposureService) reportOrphanedExposures(ctx context.Context, exposures []domain.Exposure) {
	if len(exposures) == 0 {
		return
	}

	events := make([]ExposureOrphanedEvent, 0, len(exposures))
	for _, e := range exposures {
		log.Printf("exposure %s references user %s which could not be resolved", e.ID(), e.UserID())
		events = append(events, ExposureOrphanedEvent{
			ExposureID: e.ID(),
			UserID:     e.UserID(),
			DetectedAt: time.Now().UTC(),
		})
	}

	if err := s.publisher.PublishOrphaned(ctx, events); err != nil {
		log.Printf("failed to publish %d orphaned-exposure event(s): %v", len(events), err)
	}
}

// GetUserExposureSummary sums a user's exposures within an optional time
// window. A nil from/to means that bound is unset.
func (s *ExposureService) GetUserExposureSummary(ctx context.Context, userID uuid.UUID, from, to *time.Time) (domain.ExposureSummary, error) {
	user, err := s.users.Get(ctx, userID)
	if err != nil {
		return domain.ExposureSummary{}, err
	}

	acc, err := s.exposures.SummarizeByUser(ctx, userID, from, to)
	if err != nil {
		return domain.ExposureSummary{}, err
	}

	return domain.FinalizeExposureSummary(user, acc), nil
}
