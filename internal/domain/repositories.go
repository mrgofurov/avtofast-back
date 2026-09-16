package domain

import (
	"context"
	"time"
)

type UserRepository interface {
	GetByID(ctx context.Context, id int64) (*User, error)
	GetByPublicID(ctx context.Context, publicID string) (*User, error)
	GetOrCreateByProvider(ctx context.Context, provider, providerID, email, phone, displayName string) (*User, error)
	Update(ctx context.Context, user *User) error
	// Delete erases the account and everything hanging off it. Required by
	// App Store Review Guideline 5.1.1(v): deletion has to be a real delete,
	// reachable from inside the app, not a deactivation.
	DeleteUser(ctx context.Context, userID int64) error
	
	GetOnboarding(ctx context.Context, userID int64) (*UserOnboarding, error)
	SaveOnboarding(ctx context.Context, onboarding *UserOnboarding) error
	
	GetPreferences(ctx context.Context, userID int64) (*UserPreferences, error)
	UpdatePreferences(ctx context.Context, prefs *UserPreferences) error
	
	GetNotificationPreferences(ctx context.Context, userID int64) (*NotificationPreferences, error)
	UpdateNotificationPreferences(ctx context.Context, prefs *NotificationPreferences) error
}

type DeviceRepository interface {
	UpsertDevice(ctx context.Context, device *Device) error
	GetDeviceByPublicID(ctx context.Context, publicID string) (*Device, error)
	DeleteDeviceToken(ctx context.Context, publicID string) error
}

type EntitlementRepository interface {
	GetByUserID(ctx context.Context, userID int64) (*Entitlement, error)
	SaveEntitlement(ctx context.Context, ent *Entitlement) error
	RecordPurchase(ctx context.Context, userID int64, platform, transactionID, productID, rawPayload string) error
}

type ContentRepository interface {
	GetActivePack(ctx context.Context) (*QuestionPack, error)
	GetAllPacks(ctx context.Context) ([]*QuestionPack, error)
	GetPackByID(ctx context.Context, packID string) (*QuestionPack, error)
	GetQuestionsByPack(ctx context.Context, packID, category string, cursor string, limit int) ([]*Question, string, error)
	GetQuestionByID(ctx context.Context, questionPublicID string) (*Question, error)
	GetQuestionsByIDs(ctx context.Context, questionPublicIDs []string) ([]*Question, error)
	GetRandomQuestions(ctx context.Context, packID string, count int, category string) ([]*Question, error)
	
	// Admin operations
	CreatePack(ctx context.Context, pack *QuestionPack) error
	UpdatePack(ctx context.Context, pack *QuestionPack) error
	CreateQuestion(ctx context.Context, q *Question) error
	UpdateQuestion(ctx context.Context, q *Question) error
}

type PracticeRepository interface {
	CreateSession(ctx context.Context, session *PracticeSession) error
	GetSessionByPublicID(ctx context.Context, publicID string) (*PracticeSession, error)
	SaveAnswer(ctx context.Context, sessionID int64, questionID int64, selectedChoiceID string, isCorrect bool, elapsedMs int, answeredAt time.Time) error
	CompleteSession(ctx context.Context, sessionID int64, answeredCount, correctCount int, completedAt time.Time) error
	// GetSessionTally counts what was actually answered in a session, so
	// completion records the learner's real score instead of assuming every
	// question was reached.
	GetSessionTally(ctx context.Context, sessionID int64) (answered int, correct int, err error)
}

type MockExamRepository interface {
	CreateExam(ctx context.Context, exam *MockExam) error
	GetExamByPublicID(ctx context.Context, publicID string) (*MockExam, error)
	SaveExamAnswer(ctx context.Context, examID int64, questionID int64, selectedChoiceID string, elapsedMs int) error
	// GetExamAnswers returns what the learner actually selected, keyed by
	// question id. Grading reads this: an exam is scored from the stored
	// answers, never from the questions alone.
	GetExamAnswers(ctx context.Context, examID int64) (map[int64]string, error)
	// SetExamAnswerCorrectness records the grade of one answer, so a graded
	// exam can be re-read later without regrading it.
	SetExamAnswerCorrectness(ctx context.Context, examID int64, questionID int64, isCorrect bool) error
	CompleteExam(ctx context.Context, examID int64, correctCount, scorePercent int, passed bool, completedAt time.Time) error
	GetUserExams(ctx context.Context, userID int64, cursor string, limit int) ([]*MockExam, string, error)
}

type MistakeRepository interface {
	UpsertMistake(ctx context.Context, userID int64, questionID int64, isCorrect bool) error
	GetDueMistakes(ctx context.Context, userID int64, dueOnly bool, category string, cursor string, limit int) ([]*UserMistakeItem, string, error)
}

type DashboardRepository interface {
	GetUserStats(ctx context.Context, userID int64) (*UserStats, error)
	UpdateUserStats(ctx context.Context, stats *UserStats) error
	GetCategoryAccuracy(ctx context.Context, userID int64) ([]WeakCategoryInfo, error)
	GetAnalyticsProgress(ctx context.Context, userID int64, days int) (map[string]any, error)
	// GetDailyActivity returns one entry per day over the window, oldest
	// first, with days the learner did nothing included as zeroes so the
	// chart has an unbroken axis.
	GetDailyActivity(ctx context.Context, userID int64, days int) ([]DailyActivity, error)
}

type SyncRepository interface {
	SaveSyncEvent(ctx context.Context, event *SyncEventItem, userID int64) error
	GetChangesSince(ctx context.Context, userID int64, cursor string) (*SyncChangesResponse, error)
}

type AuditRepository interface {
	LogAction(ctx context.Context, log *AuditLog) error
	GetAuditLogs(ctx context.Context, cursor string, limit int) ([]*AuditLog, string, error)
}

type IdempotencyStore interface {
	Get(ctx context.Context, key string) ([]byte, bool, error)
	Set(ctx context.Context, key string, responseData []byte, ttl time.Duration) error
}

type RateLimiterStore interface {
	Allow(ctx context.Context, key string, limit int, window time.Duration) (bool, int, time.Duration, error)
}

type CacheStore interface {
	CacheGet(ctx context.Context, key string) ([]byte, error)
	CacheSet(ctx context.Context, key string, val []byte, ttl time.Duration) error
	CacheDelete(ctx context.Context, key string) error
}
