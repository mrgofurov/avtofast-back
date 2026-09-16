package usecase

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/avtofast/avtofast-back/internal/domain"
)

// Event types a client may upload.
const (
	// SyncEventPracticeAnswer is one question answered while offline.
	SyncEventPracticeAnswer = "practice_answer"
)

type SyncUsecase struct {
	syncRepo    domain.SyncRepository
	contentRepo domain.ContentRepository
	mistakeRepo domain.MistakeRepository
	dashRepo    domain.DashboardRepository
}

func NewSyncUsecase(
	syncRepo domain.SyncRepository,
	contentRepo domain.ContentRepository,
	mistakeRepo domain.MistakeRepository,
	dashRepo domain.DashboardRepository,
) *SyncUsecase {
	return &SyncUsecase{
		syncRepo:    syncRepo,
		contentRepo: contentRepo,
		mistakeRepo: mistakeRepo,
		dashRepo:    dashRepo,
	}
}

type SyncUploadRequest struct {
	Events []domain.SyncEventItem `json:"events"`
}

// practiceAnswerPayload is what a client sends for an answer it made offline.
//
// It reports what was *selected*, never whether it was right: correctness is
// decided here against the server's own copy of the question, exactly as it is
// for an answer submitted online.
type practiceAnswerPayload struct {
	QuestionID       string `json:"questionId"`
	SelectedChoiceID string `json:"selectedChoiceId"`
	ElapsedMs        int    `json:"elapsedMs"`
}

func (u *SyncUsecase) ProcessEvents(ctx context.Context, userID int64, req SyncUploadRequest) (*domain.SyncUploadResponse, error) {
	results := make([]domain.SyncResultEventStatus, 0, len(req.Events))

	for i := range req.Events {
		evt := req.Events[i]

		isNew, err := u.syncRepo.SaveSyncEvent(ctx, &evt, userID)
		if err != nil {
			results = append(results, domain.SyncResultEventStatus{
				ID:       evt.ID,
				Accepted: false,
				Reason:   err.Error(),
			})
			continue
		}

		// An event already stored is accepted again without being applied:
		// the client retried an upload it never saw the answer to, and
		// crediting the same answer twice would inflate the learner's stats.
		if isNew {
			if err := u.apply(ctx, userID, &evt); err != nil {
				results = append(results, domain.SyncResultEventStatus{
					ID:       evt.ID,
					Accepted: false,
					Reason:   err.Error(),
				})
				continue
			}
		}

		results = append(results, domain.SyncResultEventStatus{
			ID:       evt.ID,
			Accepted: true,
		})
	}

	newCursor := fmt.Sprintf("cur_%d", time.Now().UTC().UnixNano())
	return &domain.SyncUploadResponse{
		ProcessedEvents: results,
		SyncCursor:      newCursor,
	}, nil
}

// apply folds an offline event into the learner's progress, so work done
// without a connection counts for exactly what it would have online.
//
// An event type this server does not know is stored and ignored rather than
// rejected: an older server should not fail an upload from a newer client.
func (u *SyncUsecase) apply(ctx context.Context, userID int64, evt *domain.SyncEventItem) error {
	if evt.Type != SyncEventPracticeAnswer {
		return nil
	}

	var payload practiceAnswerPayload
	if err := json.Unmarshal(evt.Payload, &payload); err != nil {
		return fmt.Errorf("invalid %s payload", evt.Type)
	}
	if payload.QuestionID == "" || payload.SelectedChoiceID == "" {
		return fmt.Errorf("%s requires questionId and selectedChoiceId", evt.Type)
	}

	question, err := u.contentRepo.GetQuestionByID(ctx, payload.QuestionID)
	if err != nil || question == nil {
		return fmt.Errorf("unknown question %s", payload.QuestionID)
	}

	isCorrect := question.CorrectChoiceID == payload.SelectedChoiceID
	_ = u.mistakeRepo.UpsertMistake(ctx, userID, question.ID, isCorrect)

	stats, _ := u.dashRepo.GetUserStats(ctx, userID)
	if stats == nil {
		stats = &domain.UserStats{UserID: userID, Level: 1}
	}
	stats.TotalAnswered++
	if isCorrect {
		stats.TotalCorrect++
		stats.XP += xpPerCorrectAnswer
	} else {
		stats.XP += xpPerIncorrectAnswer
	}
	stats.Level = stats.XP/200 + 1
	stats.StreakDays = nextStreak(stats.StreakDays, stats.LastActivityDate, time.Now().UTC())

	return u.dashRepo.UpdateUserStats(ctx, stats)
}

func (u *SyncUsecase) GetChanges(ctx context.Context, userID int64, cursor string) (*domain.SyncChangesResponse, error) {
	return u.syncRepo.GetChangesSince(ctx, userID, cursor)
}
