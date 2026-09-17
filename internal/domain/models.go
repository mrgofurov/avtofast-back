package domain

import (
	"encoding/json"
	"strings"
	"time"
)

// Allowed constants
const (
	LocaleUzLatn = "uz-Latn-UZ"
	LocaleUzCyrl = "uz-Cyrl-UZ"
	LocaleRu     = "ru"
	LocaleEn     = "en"

	CategoryRoadSigns     = "road_signs"
	CategoryTrafficRules  = "traffic_rules"
	CategoryIntersections = "intersections"
	CategoryFirstAid      = "first_aid"
	CategoryPenalties     = "penalties"
	CategoryVehicleSafety = "vehicle_safety"
	CategorySituations    = "situations"

	DifficultyEasy   = "easy"
	DifficultyMedium = "medium"
	DifficultyHard   = "hard"

	RoleUser             = "user"
	RoleContentAdmin     = "content_admin"
	RoleContentPublisher = "content_publisher"

	TierFree    = "free"
	TierPremium = "premium"

	ExamStatusInProgress = "in_progress"
	ExamStatusCompleted  = "completed"
	ExamStatusExpired    = "expired"

	PracticeModeAdaptive      = "adaptive"
	PracticeModeCategory      = "category"
	PracticeModeMistakeReview = "mistake_review"
	PracticeModeDailyGoal     = "daily_goal"

	// QuestionsPerTest is how many questions one numbered topic test holds.
	// It matches the official exam's length, so a test is a rehearsal of the
	// real thing rather than an arbitrary batch. Changing it renumbers every
	// test in every topic, which is why it is a constant and not a parameter.
	QuestionsPerTest = 20
)

// Categories is the syllabus, in the order the app lists it.
var Categories = []string{
	CategoryRoadSigns,
	CategoryTrafficRules,
	CategoryIntersections,
	CategoryFirstAid,
	CategoryPenalties,
	CategoryVehicleSafety,
	CategorySituations,
}

// IsKnownCategory reports whether a category name is one of the seven. Used to
// turn an unrecognised path parameter into a 400 rather than an empty list
// that looks like a topic with no questions in it.
func IsKnownCategory(category string) bool {
	for _, known := range Categories {
		if known == category {
			return true
		}
	}
	return false
}

type User struct {
	ID          int64     `json:"-"`
	PublicID    string    `json:"id"`
	ProviderID  string    `json:"-"`
	Provider    string    `json:"-"`
	Email       string    `json:"email,omitempty"`
	Phone       string    `json:"phone,omitempty"`
	DisplayName string    `json:"displayName"`
	AvatarURL   *string   `json:"avatarUrl"`
	Role        string    `json:"role"`
	CreatedAt   time.Time `json:"createdAt"`
	UpdatedAt   time.Time `json:"updatedAt"`
}

type UserOnboarding struct {
	ID                int64  `json:"-"`
	UserID            int64  `json:"-"`
	AcquisitionSource string `json:"acquisitionSource"`
	KnowledgeLevel    string `json:"knowledgeLevel"`
	Locale            string `json:"locale"`
	TargetExamDate    string `json:"targetExamDate"`
	DailyQuestionGoal int    `json:"dailyQuestionGoal"`
	Completed         bool   `json:"completed"`
}

type UserPreferences struct {
	ID                    int64  `json:"-"`
	UserID                int64  `json:"-"`
	Locale                string `json:"locale"`
	Theme                 string `json:"theme"`
	ProfileVisibility     string `json:"profileVisibility"`
	LeaderboardVisibility string `json:"leaderboardVisibility"`
}

type NotificationPreferences struct {
	ID               int64  `json:"-"`
	UserID           int64  `json:"-"`
	StudyReminder    bool   `json:"studyReminder"`
	StreakProtection bool   `json:"streakProtection"`
	MistakeReview    bool   `json:"mistakeReview"`
	WeeklySummary    bool   `json:"weeklySummary"`
	ScoreImprovement bool   `json:"scoreImprovement"`
	ReminderTime     string `json:"reminderTime"`
	Timezone         string `json:"timezone"`
}

type Entitlement struct {
	ID        int64           `json:"-"`
	UserID    int64           `json:"-"`
	Tier      string          `json:"tier"`
	Status    string          `json:"status"`
	ProductID *string         `json:"productId"`
	ExpiresAt *time.Time      `json:"expiresAt"`
	Features  map[string]bool `json:"features"`
}

type Device struct {
	ID         int64   `json:"-"`
	PublicID   string  `json:"deviceId"`
	UserID     int64   `json:"-"`
	Platform   string  `json:"platform"`
	PushToken  *string `json:"pushToken"`
	AppVersion string  `json:"appVersion"`
	Locale     string  `json:"locale"`
	Timezone   string  `json:"timezone"`
}

type ChoiceItem struct {
	ID       string `json:"id"`
	Text     string `json:"text"`
	Position int    `json:"position"`
}

type QuestionImage struct {
	URL    string            `json:"url"`
	SHA256 string            `json:"sha256"`
	Alt    map[string]string `json:"alt,omitempty"`
}

type QuestionSource struct {
	Reference         string `json:"reference"`
	EffectiveFrom     string `json:"effectiveFrom,omitempty"`
	OfficialSourceURL string `json:"officialSourceUrl,omitempty"`
}

type QuestionTranslationData struct {
	Prompt      string       `json:"prompt"`
	Choices     []ChoiceItem `json:"choices"`
	Explanation string       `json:"explanation"`
}

type Question struct {
	ID              int64                              `json:"-"`
	PublicID        string                             `json:"id"`
	PackTableID     int64                              `json:"-"`
	PackID          string                             `json:"packId"`
	ContentVersion  string                             `json:"contentVersion"`
	Category        string                             `json:"category"`
	Difficulty      string                             `json:"difficulty"`
	Image           *QuestionImage                     `json:"image,omitempty"`
	VideoURL        string                             `json:"videoUrl,omitempty"`
	AudioURL        string                             `json:"audioUrl,omitempty"`
	ExternalID      int64                              `json:"externalId,omitempty"`
	Source          *QuestionSource                    `json:"source,omitempty"`
	CorrectChoiceID string                             `json:"-"` // never exposed in online question payload
	Translations    map[string]QuestionTranslationData `json:"translations,omitempty"`
	Status          string                             `json:"status"`
}

// ClientQuestionPayload strips out correct answers and returns targeted translation
type ClientQuestionPayload struct {
	ID             string          `json:"id"`
	PackId         string          `json:"packId"`
	ContentVersion string          `json:"contentVersion"`
	Category       string          `json:"category"`
	Difficulty     string          `json:"difficulty"`
	Image          *QuestionImage  `json:"image,omitempty"`
	VideoURL       string          `json:"videoUrl,omitempty"`
	AudioURL       string          `json:"audioUrl,omitempty"`
	Source         *QuestionSource `json:"source,omitempty"`
	Prompt         string          `json:"prompt"`
	Choices        []ChoiceItem    `json:"choices"`
}

// MediaMountPath is where question media is served from. Media is stored with
// a bare relative path ("questions/<uuid>.webp"), which on its own is not
// resolvable by a client: joined to the API origin it would miss this mount.
// The path is therefore prefixed on the way out, so every client only has to
// join the origin to what it is given.
const MediaMountPath = "/medias/"

// PublicMediaPath makes a stored media path resolvable against the API origin.
// An absolute URL or an already-prefixed path is returned unchanged, so this
// stays safe to apply more than once.
func PublicMediaPath(path string) string {
	if path == "" {
		return ""
	}
	if strings.HasPrefix(path, "http://") || strings.HasPrefix(path, "https://") {
		return path
	}
	if strings.HasPrefix(path, MediaMountPath) || strings.HasPrefix(path, "/uploads/") {
		return path
	}
	return MediaMountPath + strings.TrimPrefix(path, "/")
}

func (q *Question) ToClientPayload(locale string) ClientQuestionPayload {
	tr, ok := q.Translations[locale]
	if !ok {
		// fallback to uz-Latn-UZ
		tr = q.Translations[LocaleUzLatn]
	}

	image := q.Image
	if image != nil {
		// Copied rather than mutated: the stored question is shared between
		// concurrent requests, and rewriting its URL in place would prefix it
		// again on every one of them.
		withPublicURL := *image
		withPublicURL.URL = PublicMediaPath(image.URL)
		image = &withPublicURL
	}

	return ClientQuestionPayload{
		ID:             q.PublicID,
		PackId:         q.PackID,
		ContentVersion: q.ContentVersion,
		Category:       q.Category,
		Difficulty:     q.Difficulty,
		Image:          image,
		VideoURL:       PublicMediaPath(q.VideoURL),
		AudioURL:       PublicMediaPath(q.AudioURL),
		Source:         q.Source,
		Prompt:         tr.Prompt,
		Choices:        tr.Choices,
	}
}

type QuestionPack struct {
	ID                int64      `json:"-"`
	PackID            string     `json:"id"`
	Version           string     `json:"version"`
	Title             string     `json:"title"`
	Locales           []string   `json:"locales"`
	QuestionCount     int        `json:"questionCount"`
	DownloadBytes     int64      `json:"downloadBytes"`
	MandatoryUpdate   bool       `json:"mandatoryUpdate"`
	Status            string     `json:"status"`
	ManifestSHA256    string     `json:"manifestSha256,omitempty"`
	ManifestSignature string     `json:"manifestSignature,omitempty"`
	PublishedAt       *time.Time `json:"publishedAt,omitempty"`
}

type PracticeSession struct {
	ID          int64   `json:"-"`
	PublicID    string  `json:"id"`
	UserID      int64   `json:"-"`
	Mode        string  `json:"mode"`
	PackID      string  `json:"packId"`
	PackVersion string  `json:"packVersion"`
	Locale      string  `json:"locale"`
	Category    *string `json:"category,omitempty"`
	// TestIndex is the 1-based numbered test this session was, for a category
	// session that asked for one. Nil for every other mode, and for the
	// free-form category practice that predates numbered tests.
	TestIndex      *int        `json:"testIndex,omitempty"`
	Status         string      `json:"status"`
	TotalQuestions int         `json:"totalQuestions"`
	AnsweredCount  int         `json:"answeredCount"`
	CorrectCount   int         `json:"correctCount"`
	CreatedAt      time.Time   `json:"createdAt"`
	CompletedAt    *time.Time  `json:"completedAt,omitempty"`
	Questions      []*Question `json:"-"`
}

type PracticeAnswerSubmission struct {
	QuestionID       string    `json:"questionId"`
	SelectedChoiceID string    `json:"selectedChoiceId"`
	ElapsedMs        int       `json:"elapsedMs"`
	AnsweredAt       time.Time `json:"answeredAt"`
}

type PracticeAnswerResult struct {
	QuestionID        string     `json:"questionId"`
	IsCorrect         bool       `json:"isCorrect"`
	CorrectChoiceID   string     `json:"correctChoiceId"`
	Explanation       string     `json:"explanation"`
	Reference         string     `json:"reference"`
	ExplanationAccess string     `json:"explanationAccess"` // full | preview
	XpAwarded         int        `json:"xpAwarded"`
	ReviewScheduledAt *time.Time `json:"reviewScheduledAt"`
}

type MockExam struct {
	ID               int64       `json:"-"`
	PublicID         string      `json:"id"`
	UserID           int64       `json:"-"`
	PackID           string      `json:"packId"`
	PackVersion      string      `json:"packVersion"`
	Locale           string      `json:"locale"`
	Format           string      `json:"format"`
	Status           string      `json:"status"`
	StartedAt        time.Time   `json:"startedAt"`
	DeadlineAt       time.Time   `json:"deadlineAt"`
	QuestionCount    int         `json:"questionCount"`
	PassCorrectCount int         `json:"passCorrectCount"`
	CorrectCount     int         `json:"correctCount"`
	ScorePercent     int         `json:"scorePercent"`
	Passed           bool        `json:"passed"`
	CompletedAt      *time.Time  `json:"completedAt,omitempty"`
	Questions        []*Question `json:"-"`
}

type MockExamQuestionAnswer struct {
	SelectedChoiceID string `json:"selectedChoiceId"`
	ElapsedMs        int    `json:"elapsedMs"`
}

type MockExamQuestionReview struct {
	QuestionID       string `json:"questionId"`
	SelectedChoiceID string `json:"selectedChoiceId"`
	CorrectChoiceID  string `json:"correctChoiceId"`
	IsCorrect        bool   `json:"isCorrect"`
	Explanation      string `json:"explanation"`
	Reference        string `json:"reference"`
}

type MockExamSubmissionResult struct {
	Status           string                   `json:"status"`
	CorrectCount     int                      `json:"correctCount"`
	QuestionCount    int                      `json:"questionCount"`
	PassCorrectCount int                      `json:"passCorrectCount"`
	Passed           bool                     `json:"passed"`
	ScorePercent     int                      `json:"scorePercent"`
	CompletedAt      time.Time                `json:"completedAt"`
	Review           []MockExamQuestionReview `json:"review"`
}

type UserMistakeItem struct {
	Question        ClientQuestionPayload `json:"question"`
	MistakeCount    int                   `json:"mistakeCount"`
	LastIncorrectAt time.Time             `json:"lastIncorrectAt"`
	NextReviewAt    time.Time             `json:"nextReviewAt"`
	Priority        string                `json:"priority"` // high, medium, low
}

type UserStats struct {
	ID                 int64     `json:"-"`
	UserID             int64     `json:"-"`
	XP                 int       `json:"xp"`
	Level              int       `json:"level"`
	StreakDays         int       `json:"streakDays"`
	LastActivityDate   *string   `json:"lastActivityDate,omitempty"`
	TotalAnswered      int       `json:"totalAnswered"`
	TotalCorrect       int       `json:"totalCorrect"`
	CompletedMockExams int       `json:"completedMockExams"`
	PassedMockExams    int       `json:"passedMockExams"`
	UpdatedAt          time.Time `json:"-"`
}

// TopicSummary is one row of the topics list: how much of a category exists,
// how it is cut into numbered tests, and how far the learner has got through
// them.
type TopicSummary struct {
	Category      string `json:"category"`
	QuestionCount int    `json:"questionCount"`
	TestCount     int    `json:"testCount"`
	// CompletedTests counts tests attempted through to completion at least
	// once — the honest measure of coverage, unlike accuracy, which says
	// nothing about how much of the topic has been seen.
	CompletedTests  int `json:"completedTests"`
	AccuracyPercent int `json:"accuracyPercent"`
	DueMistakes     int `json:"dueMistakes"`
}

// TopicTest is one numbered test and what the learner has scored on it.
//
// BestCorrect is the best attempt rather than the last: a test the learner
// once got 18/20 on has been learned, and showing a worse retry in its place
// would punish practising.
type TopicTest struct {
	Index         int        `json:"index"`
	QuestionCount int        `json:"questionCount"`
	Attempts      int        `json:"attempts"`
	BestCorrect   *int       `json:"bestCorrect,omitempty"`
	LastAttemptAt *time.Time `json:"lastAttemptAt,omitempty"`
}

type WeakCategoryInfo struct {
	Category        string `json:"category"`
	AccuracyPercent int    `json:"accuracyPercent"`
	DueMistakes     int    `json:"dueMistakes"`
}

type DashboardSnapshot struct {
	XP                 int             `json:"xp"`
	Level              int             `json:"level"`
	StreakDays         int             `json:"streakDays"`
	DailyGoal          DailyGoalStatus `json:"dailyGoal"`
	ReadinessScore     int             `json:"readinessScore"`
	AccuracyPercent    int             `json:"accuracyPercent"`
	CompletedMockExams int             `json:"completedMockExams"`
	// DueMistakeCount is what the home screen's mistakes tile counts, so the
	// number there and the number on the review screen come from one place.
	DueMistakeCount       int                `json:"dueMistakeCount"`
	WeakCategories        []WeakCategoryInfo `json:"weakCategories"`
	NextRecommendedAction string             `json:"nextRecommendedAction"`
}

type DailyGoalStatus struct {
	Completed int `json:"completed"`
	Target    int `json:"target"`
}

// DailyActivity is one day of answering, for the progress chart and for
// deciding how much of today's goal is already done.
type DailyActivity struct {
	Date     string `json:"date"`
	Answered int    `json:"answered"`
	Correct  int    `json:"correct"`
}

type SyncEventItem struct {
	ID          string          `json:"id"`
	Type        string          `json:"type"`
	OccurredAt  time.Time       `json:"occurredAt"`
	PackID      string          `json:"packId"`
	PackVersion string          `json:"packVersion"`
	Payload     json.RawMessage `json:"payload"`
}

type SyncResultEventStatus struct {
	ID       string `json:"id"`
	Accepted bool   `json:"accepted"`
	Reason   string `json:"reason,omitempty"`
}

type SyncUploadResponse struct {
	ProcessedEvents []SyncResultEventStatus `json:"processedEvents"`
	SyncCursor      string                  `json:"syncCursor"`
}

type SyncChangesResponse struct {
	ProfileChanges   any    `json:"profileChanges,omitempty"`
	Entitlement      any    `json:"entitlement,omitempty"`
	ReviewQueueCount int    `json:"reviewQueueCount"`
	Dashboard        any    `json:"dashboard,omitempty"`
	NextCursor       string `json:"nextCursor"`
}

type AuditLog struct {
	ID          int64     `json:"-"`
	PublicID    string    `json:"id"`
	ActorID     int64     `json:"-"`
	ActorName   string    `json:"actor"`
	Action      string    `json:"action"`
	TargetType  string    `json:"targetType"`
	TargetID    string    `json:"targetId"`
	BeforeState any       `json:"beforeState,omitempty"`
	AfterState  any       `json:"afterState,omitempty"`
	Reason      string    `json:"reason"`
	CreatedAt   time.Time `json:"createdAt"`
}
