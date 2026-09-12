package memory

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"sync"
	"time"

	"github.com/avtofast/avtofast-back/internal/domain"
	"github.com/avtofast/avtofast-back/pkg/id"
)

type MemoryStore struct {
	mu sync.RWMutex

	// Auto-increment sequence for BIGSERIAL IDs
	userSeq     int64
	packSeq     int64
	questionSeq int64
	sessionSeq  int64
	examSeq     int64
	mistakeSeq  int64
	deviceSeq   int64
	auditSeq    int64

	Users               map[int64]*domain.User
	UsersByPublicID     map[string]*domain.User
	UsersByProvider     map[string]*domain.User // "provider:providerID"
	Onboardings         map[int64]*domain.UserOnboarding
	Preferences         map[int64]*domain.UserPreferences
	NotificationPrefs   map[int64]*domain.NotificationPreferences
	Devices             map[string]*domain.Device // publicID -> device
	Entitlements        map[int64]*domain.Entitlement
	Purchases           map[string]map[string]any // transactionID -> data
	Packs               map[string]*domain.QuestionPack // packID -> pack
	Questions           map[string]*domain.Question     // publicID -> question
	PracticeSessions    map[string]*domain.PracticeSession // publicID -> session
	PracticeQuestions   map[int64][]*domain.Question       // sessionID -> questions
	PracticeAnswers     map[string]map[string]any          // "sessionID:qID" -> answer
	MockExams           map[string]*domain.MockExam        // publicID -> exam
	MockExamQuestions   map[int64][]*domain.Question       // examID -> questions
	MockExamAnswers     map[string]map[string]any          // "examID:qID" -> answer
	UserMistakes        map[string]*domain.UserMistakeItem // "userID:qID" -> mistake
	UserStats           map[int64]*domain.UserStats
	SyncEvents          map[string]*domain.SyncEventItem
	AuditLogs           []*domain.AuditLog

	// Caching & Idempotency
	Idempotency map[string][]byte
	Cache       map[string][]byte
	RateLimits  map[string]int
}

func New() *MemoryStore {
	return &MemoryStore{
		Users:             make(map[int64]*domain.User),
		UsersByPublicID:   make(map[string]*domain.User),
		UsersByProvider:   make(map[string]*domain.User),
		Onboardings:       make(map[int64]*domain.UserOnboarding),
		Preferences:       make(map[int64]*domain.UserPreferences),
		NotificationPrefs: make(map[int64]*domain.NotificationPreferences),
		Devices:           make(map[string]*domain.Device),
		Entitlements:      make(map[int64]*domain.Entitlement),
		Purchases:         make(map[string]map[string]any),
		Packs:             make(map[string]*domain.QuestionPack),
		Questions:         make(map[string]*domain.Question),
		PracticeSessions:  make(map[string]*domain.PracticeSession),
		PracticeQuestions: make(map[int64][]*domain.Question),
		PracticeAnswers:   make(map[string]map[string]any),
		MockExams:         make(map[string]*domain.MockExam),
		MockExamQuestions: make(map[int64][]*domain.Question),
		MockExamAnswers:   make(map[string]map[string]any),
		UserMistakes:      make(map[string]*domain.UserMistakeItem),
		UserStats:         make(map[int64]*domain.UserStats),
		SyncEvents:        make(map[string]*domain.SyncEventItem),
		AuditLogs:         make([]*domain.AuditLog, 0),
		Idempotency:       make(map[string][]byte),
		Cache:             make(map[string][]byte),
		RateLimits:        make(map[string]int),
	}
}

// Ensure interfaces are satisfied
var _ domain.UserRepository = (*MemoryStore)(nil)
var _ domain.DeviceRepository = (*MemoryStore)(nil)
var _ domain.EntitlementRepository = (*MemoryStore)(nil)
var _ domain.ContentRepository = (*MemoryStore)(nil)
var _ domain.PracticeRepository = (*MemoryStore)(nil)
var _ domain.MockExamRepository = (*MemoryStore)(nil)
var _ domain.MistakeRepository = (*MemoryStore)(nil)
var _ domain.DashboardRepository = (*MemoryStore)(nil)
var _ domain.SyncRepository = (*MemoryStore)(nil)
var _ domain.AuditRepository = (*MemoryStore)(nil)
var _ domain.IdempotencyStore = (*MemoryStore)(nil)
var _ domain.RateLimiterStore = (*MemoryStore)(nil)
var _ domain.CacheStore = (*MemoryStore)(nil)

// User Repository
func (m *MemoryStore) GetByID(ctx context.Context, id int64) (*domain.User, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.Users[id], nil
}

func (m *MemoryStore) GetByPublicID(ctx context.Context, publicID string) (*domain.User, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.UsersByPublicID[publicID], nil
}

func (m *MemoryStore) GetOrCreateByProvider(ctx context.Context, provider, providerID, email, phone, displayName string) (*domain.User, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	key := fmt.Sprintf("%s:%s", provider, providerID)
	if u, ok := m.UsersByProvider[key]; ok {
		return u, nil
	}

	m.userSeq++
	pubID := id.New(id.PrefixUser)
	now := time.Now().UTC()
	u := &domain.User{
		ID:          m.userSeq,
		PublicID:    pubID,
		ProviderID:  providerID,
		Provider:    provider,
		Email:       email,
		Phone:       phone,
		DisplayName: displayName,
		Role:        domain.RoleUser,
		CreatedAt:   now,
		UpdatedAt:   now,
	}

	m.Users[u.ID] = u
	m.UsersByPublicID[pubID] = u
	m.UsersByProvider[key] = u

	// Default user stats
	m.UserStats[u.ID] = &domain.UserStats{
		ID:     u.ID,
		UserID: u.ID,
		XP:     0,
		Level:  1,
	}

	// Default preferences
	m.Preferences[u.ID] = &domain.UserPreferences{
		ID:                    u.ID,
		UserID:                u.ID,
		Locale:                domain.LocaleUzLatn,
		Theme:                 "system",
		ProfileVisibility:     "friends",
		LeaderboardVisibility: "friends",
	}

	// Default notifications
	m.NotificationPrefs[u.ID] = &domain.NotificationPreferences{
		ID:               u.ID,
		UserID:           u.ID,
		StudyReminder:    true,
		StreakProtection: true,
		MistakeReview:    true,
		WeeklySummary:    true,
		ScoreImprovement: true,
		ReminderTime:     "19:00",
		Timezone:         "Asia/Tashkent",
	}

	// Default entitlement
	m.Entitlements[u.ID] = &domain.Entitlement{
		ID:       u.ID,
		UserID:   u.ID,
		Tier:     domain.TierFree,
		Status:   "active",
		Features: map[string]bool{"ads": true},
	}

	return u, nil
}

func (m *MemoryStore) Update(ctx context.Context, user *domain.User) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	user.UpdatedAt = time.Now().UTC()
	m.Users[user.ID] = user
	m.UsersByPublicID[user.PublicID] = user
	return nil
}

func (m *MemoryStore) GetOnboarding(ctx context.Context, userID int64) (*domain.UserOnboarding, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.Onboardings[userID], nil
}

func (m *MemoryStore) SaveOnboarding(ctx context.Context, onboarding *domain.UserOnboarding) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	onboarding.Completed = true
	m.Onboardings[onboarding.UserID] = onboarding
	return nil
}

func (m *MemoryStore) GetPreferences(ctx context.Context, userID int64) (*domain.UserPreferences, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if p, ok := m.Preferences[userID]; ok {
		return p, nil
	}
	return &domain.UserPreferences{
		UserID:                userID,
		Locale:                domain.LocaleUzLatn,
		Theme:                 "system",
		ProfileVisibility:     "friends",
		LeaderboardVisibility: "friends",
	}, nil
}

func (m *MemoryStore) UpdatePreferences(ctx context.Context, prefs *domain.UserPreferences) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.Preferences[prefs.UserID] = prefs
	return nil
}

func (m *MemoryStore) GetNotificationPreferences(ctx context.Context, userID int64) (*domain.NotificationPreferences, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if np, ok := m.NotificationPrefs[userID]; ok {
		return np, nil
	}
	return &domain.NotificationPreferences{
		UserID:           userID,
		StudyReminder:    true,
		StreakProtection: true,
		MistakeReview:    true,
		WeeklySummary:    true,
		ScoreImprovement: true,
		ReminderTime:     "19:00",
		Timezone:         "Asia/Tashkent",
	}, nil
}

func (m *MemoryStore) UpdateNotificationPreferences(ctx context.Context, prefs *domain.NotificationPreferences) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.NotificationPrefs[prefs.UserID] = prefs
	return nil
}

// Devices
func (m *MemoryStore) UpsertDevice(ctx context.Context, device *domain.Device) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.Devices[device.PublicID] = device
	return nil
}

func (m *MemoryStore) GetDeviceByPublicID(ctx context.Context, publicID string) (*domain.Device, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.Devices[publicID], nil
}

func (m *MemoryStore) DeleteDeviceToken(ctx context.Context, publicID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if d, ok := m.Devices[publicID]; ok {
		d.PushToken = nil
	}
	return nil
}

// Entitlements
func (m *MemoryStore) GetByUserID(ctx context.Context, userID int64) (*domain.Entitlement, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if ent, ok := m.Entitlements[userID]; ok {
		return ent, nil
	}
	return &domain.Entitlement{
		UserID:   userID,
		Tier:     domain.TierFree,
		Status:   "active",
		Features: map[string]bool{"ads": true},
	}, nil
}

func (m *MemoryStore) SaveEntitlement(ctx context.Context, ent *domain.Entitlement) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.Entitlements[ent.UserID] = ent
	return nil
}

func (m *MemoryStore) RecordPurchase(ctx context.Context, userID int64, platform, transactionID, productID, rawPayload string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.Purchases[transactionID] = map[string]any{
		"userID":    userID,
		"platform":  platform,
		"productID": productID,
		"raw":       rawPayload,
	}
	return nil
}

// Content
func (m *MemoryStore) GetActivePack(ctx context.Context) (*domain.QuestionPack, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	for _, p := range m.Packs {
		if p.Status == "published" {
			return p, nil
		}
	}
	return nil, nil
}

func (m *MemoryStore) GetAllPacks(ctx context.Context) ([]*domain.QuestionPack, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	var list []*domain.QuestionPack
	for _, p := range m.Packs {
		if p.Status == "published" {
			list = append(list, p)
		}
	}
	return list, nil
}

func (m *MemoryStore) GetPackByID(ctx context.Context, packID string) (*domain.QuestionPack, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.Packs[packID], nil
}

func (m *MemoryStore) GetQuestionsByPack(ctx context.Context, packID, category string, cursor string, limit int) ([]*domain.Question, string, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if limit <= 0 {
		limit = 20
	}
	var matched []*domain.Question
	for _, q := range m.Questions {
		if q.PackID == packID && q.Status == "published" {
			if category != "" && q.Category != category {
				continue
			}
			if cursor != "" && q.PublicID <= cursor {
				continue
			}
			matched = append(matched, q)
		}
	}
	sort.Slice(matched, func(i, j int) bool {
		return matched[i].PublicID < matched[j].PublicID
	})

	var nextCursor string
	if len(matched) > limit {
		nextCursor = matched[limit-1].PublicID
		matched = matched[:limit]
	}
	return matched, nextCursor, nil
}

func (m *MemoryStore) GetQuestionByID(ctx context.Context, questionPublicID string) (*domain.Question, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.Questions[questionPublicID], nil
}

func (m *MemoryStore) GetQuestionsByIDs(ctx context.Context, questionPublicIDs []string) ([]*domain.Question, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	var list []*domain.Question
	for _, pid := range questionPublicIDs {
		if q, ok := m.Questions[pid]; ok {
			list = append(list, q)
		}
	}
	return list, nil
}

func (m *MemoryStore) GetRandomQuestions(ctx context.Context, packID string, count int, category string) ([]*domain.Question, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	var pool []*domain.Question
	for _, q := range m.Questions {
		if q.PackID == packID && q.Status == "published" {
			if category != "" && q.Category != category {
				continue
			}
			pool = append(pool, q)
		}
	}
	if len(pool) <= count {
		return pool, nil
	}
	return pool[:count], nil
}

func (m *MemoryStore) CreatePack(ctx context.Context, pack *domain.QuestionPack) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.packSeq++
	pack.ID = m.packSeq
	m.Packs[pack.PackID] = pack
	return nil
}

func (m *MemoryStore) UpdatePack(ctx context.Context, pack *domain.QuestionPack) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.Packs[pack.PackID] = pack
	return nil
}

func (m *MemoryStore) CreateQuestion(ctx context.Context, q *domain.Question) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.questionSeq++
	q.ID = m.questionSeq
	m.Questions[q.PublicID] = q
	return nil
}

func (m *MemoryStore) UpdateQuestion(ctx context.Context, q *domain.Question) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.Questions[q.PublicID] = q
	return nil
}

// Practice
func (m *MemoryStore) CreateSession(ctx context.Context, session *domain.PracticeSession) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.sessionSeq++
	session.ID = m.sessionSeq
	m.PracticeSessions[session.PublicID] = session
	m.PracticeQuestions[session.ID] = session.Questions
	return nil
}

func (m *MemoryStore) GetSessionByPublicID(ctx context.Context, publicID string) (*domain.PracticeSession, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	s, ok := m.PracticeSessions[publicID]
	if !ok {
		return nil, nil
	}
	s.Questions = m.PracticeQuestions[s.ID]
	return s, nil
}

func (m *MemoryStore) SaveAnswer(ctx context.Context, sessionID int64, questionID int64, selectedChoiceID string, isCorrect bool, elapsedMs int, answeredAt time.Time) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	key := fmt.Sprintf("%d:%d", sessionID, questionID)
	m.PracticeAnswers[key] = map[string]any{
		"choice":     selectedChoiceID,
		"isCorrect":  isCorrect,
		"elapsedMs":  elapsedMs,
		"answeredAt": answeredAt,
	}
	return nil
}

func (m *MemoryStore) CompleteSession(ctx context.Context, sessionID int64, answeredCount, correctCount int, completedAt time.Time) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, s := range m.PracticeSessions {
		if s.ID == sessionID {
			s.Status = "completed"
			s.AnsweredCount = answeredCount
			s.CorrectCount = correctCount
			s.CompletedAt = &completedAt
			break
		}
	}
	return nil
}

// Mock Exam
func (m *MemoryStore) CreateExam(ctx context.Context, exam *domain.MockExam) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.examSeq++
	exam.ID = m.examSeq
	m.MockExams[exam.PublicID] = exam
	m.MockExamQuestions[exam.ID] = exam.Questions
	return nil
}

func (m *MemoryStore) GetExamByPublicID(ctx context.Context, publicID string) (*domain.MockExam, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	e, ok := m.MockExams[publicID]
	if !ok {
		return nil, nil
	}
	e.Questions = m.MockExamQuestions[e.ID]
	return e, nil
}

func (m *MemoryStore) SaveExamAnswer(ctx context.Context, examID int64, questionID int64, selectedChoiceID string, elapsedMs int) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	key := fmt.Sprintf("%d:%d", examID, questionID)
	m.MockExamAnswers[key] = map[string]any{
		"choice":    selectedChoiceID,
		"elapsedMs": elapsedMs,
	}
	return nil
}

func (m *MemoryStore) CompleteExam(ctx context.Context, examID int64, correctCount, scorePercent int, passed bool, completedAt time.Time) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, e := range m.MockExams {
		if e.ID == examID {
			e.Status = domain.ExamStatusCompleted
			e.CorrectCount = correctCount
			e.ScorePercent = scorePercent
			e.Passed = passed
			e.CompletedAt = &completedAt
			break
		}
	}
	return nil
}

func (m *MemoryStore) GetUserExams(ctx context.Context, userID int64, cursor string, limit int) ([]*domain.MockExam, string, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	var list []*domain.MockExam
	for _, e := range m.MockExams {
		if e.UserID == userID {
			list = append(list, e)
		}
	}
	return list, "", nil
}

// Mistakes
func (m *MemoryStore) UpsertMistake(ctx context.Context, userID int64, questionID int64, isCorrect bool) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	key := fmt.Sprintf("%d:%d", userID, questionID)
	now := time.Now().UTC()
	if !isCorrect {
		item, exists := m.UserMistakes[key]
		if !exists {
			var targetQ *domain.Question
			for _, q := range m.Questions {
				if q.ID == questionID {
					targetQ = q
					break
				}
			}
			if targetQ == nil {
				return nil
			}
			m.UserMistakes[key] = &domain.UserMistakeItem{
				Question:        targetQ.ToClientPayload(domain.LocaleUzLatn),
				MistakeCount:    1,
				LastIncorrectAt: now,
				NextReviewAt:    now.Add(24 * time.Hour),
				Priority:        "high",
			}
		} else {
			item.MistakeCount++
			item.LastIncorrectAt = now
			item.NextReviewAt = now.Add(24 * time.Hour)
			item.Priority = "high"
		}
		return nil
	}

	if item, exists := m.UserMistakes[key]; exists {
		item.NextReviewAt = now.Add(72 * time.Hour)
		item.Priority = "low"
	}
	return nil
}

func (m *MemoryStore) GetDueMistakes(ctx context.Context, userID int64, dueOnly bool, category string, cursor string, limit int) ([]*domain.UserMistakeItem, string, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	var list []*domain.UserMistakeItem
	for _, item := range m.UserMistakes {
		if category != "" && item.Question.Category != category {
			continue
		}
		list = append(list, item)
	}
	return list, "", nil
}

// Dashboard
func (m *MemoryStore) GetUserStats(ctx context.Context, userID int64) (*domain.UserStats, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if s, ok := m.UserStats[userID]; ok {
		return s, nil
	}
	return &domain.UserStats{UserID: userID, XP: 0, Level: 1}, nil
}

func (m *MemoryStore) UpdateUserStats(ctx context.Context, stats *domain.UserStats) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.UserStats[stats.UserID] = stats
	return nil
}

func (m *MemoryStore) GetCategoryAccuracy(ctx context.Context, userID int64) ([]domain.WeakCategoryInfo, error) {
	return []domain.WeakCategoryInfo{
		{Category: "intersections", AccuracyPercent: 60, DueMistakes: 2},
	}, nil
}

func (m *MemoryStore) GetAnalyticsProgress(ctx context.Context, userID int64, days int) (map[string]any, error) {
	stats, _ := m.GetUserStats(ctx, userID)
	return map[string]any{
		"rangeDays":            days,
		"overallAccuracy":      75,
		"totalQuestionsAnswered": stats.TotalAnswered,
		"completedMockExams":   stats.CompletedMockExams,
		"passedMockExams":      stats.PassedMockExams,
		"categories":           []any{},
		"streak":               stats.StreakDays,
	}, nil
}

// Sync
func (m *MemoryStore) SaveSyncEvent(ctx context.Context, event *domain.SyncEventItem, userID int64) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.SyncEvents[event.ID] = event
	return nil
}

func (m *MemoryStore) GetChangesSince(ctx context.Context, userID int64, cursor string) (*domain.SyncChangesResponse, error) {
	prefs, _ := m.GetPreferences(ctx, userID)
	ent, _ := m.GetByUserID(ctx, userID)
	stats, _ := m.GetUserStats(ctx, userID)
	return &domain.SyncChangesResponse{
		ProfileChanges:   prefs,
		Entitlement:      ent,
		ReviewQueueCount: len(m.UserMistakes),
		Dashboard:        stats,
		NextCursor:       fmt.Sprintf("cur_%d", time.Now().UnixNano()),
	}, nil
}

// Audit
func (m *MemoryStore) LogAction(ctx context.Context, log *domain.AuditLog) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.auditSeq++
	log.ID = m.auditSeq
	log.CreatedAt = time.Now().UTC()
	m.AuditLogs = append(m.AuditLogs, log)
	return nil
}

func (m *MemoryStore) GetAuditLogs(ctx context.Context, cursor string, limit int) ([]*domain.AuditLog, string, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	n := len(m.AuditLogs)
	rev := make([]*domain.AuditLog, n)
	for i, l := range m.AuditLogs {
		rev[n-1-i] = l
	}
	return rev, "", nil
}

// Caching & Idempotency
func (m *MemoryStore) Get(ctx context.Context, key string) ([]byte, bool, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	val, ok := m.Idempotency[key]
	return val, ok, nil
}

func (m *MemoryStore) Set(ctx context.Context, key string, responseData []byte, ttl time.Duration) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.Idempotency[key] = responseData
	return nil
}

func (m *MemoryStore) Allow(ctx context.Context, key string, limit int, window time.Duration) (bool, int, time.Duration, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	count := m.RateLimits[key]
	if count >= limit {
		return false, 0, window, nil
	}
	m.RateLimits[key] = count + 1
	return true, limit - (count + 1), 0, nil
}

func (m *MemoryStore) CacheGet(ctx context.Context, key string) ([]byte, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	val, ok := m.Cache[key]
	if !ok {
		return nil, errors.New("miss")
	}
	return val, nil
}

func (m *MemoryStore) CacheSet(ctx context.Context, key string, val []byte, ttl time.Duration) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.Cache[key] = val
	return nil
}

func (m *MemoryStore) CacheDelete(ctx context.Context, key string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.Cache, key)
	return nil
}

func (m *MemoryStore) Delete(ctx context.Context, key string) error {
	return m.CacheDelete(ctx, key)
}
