package usecase

import (
	"context"
	"fmt"
	"time"

	"github.com/avtofast/avtofast-back/internal/domain"
)

type SyncUsecase struct {
	syncRepo domain.SyncRepository
}

func NewSyncUsecase(syncRepo domain.SyncRepository) *SyncUsecase {
	return &SyncUsecase{syncRepo: syncRepo}
}

type SyncUploadRequest struct {
	Events []domain.SyncEventItem `json:"events"`
}

func (u *SyncUsecase) ProcessEvents(ctx context.Context, userID int64, req SyncUploadRequest) (*domain.SyncUploadResponse, error) {
	var results []domain.SyncResultEventStatus
	for _, evt := range req.Events {
		err := u.syncRepo.SaveSyncEvent(ctx, &evt, userID)
		if err != nil {
			results = append(results, domain.SyncResultEventStatus{
				ID:       evt.ID,
				Accepted: false,
				Reason:   err.Error(),
			})
		} else {
			results = append(results, domain.SyncResultEventStatus{
				ID:       evt.ID,
				Accepted: true,
			})
		}
	}

	newCursor := fmt.Sprintf("cur_%d", time.Now().UTC().UnixNano())
	return &domain.SyncUploadResponse{
		ProcessedEvents: results,
		SyncCursor:      newCursor,
	}, nil
}

func (u *SyncUsecase) GetChanges(ctx context.Context, userID int64, cursor string) (*domain.SyncChangesResponse, error) {
	return u.syncRepo.GetChangesSince(ctx, userID, cursor)
}
