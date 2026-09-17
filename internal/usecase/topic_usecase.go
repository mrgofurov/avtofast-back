package usecase

import (
	"context"
	"errors"

	"github.com/avtofast/avtofast-back/internal/domain"
)

// TopicUsecase serves the topics screen: the syllabus cut into numbered tests,
// and how far the learner has got through each one.
//
// A "test" here is a fixed slice of a category — questions 1–20 are Test 1,
// 21–40 are Test 2, and so on over the whole bank. Slices rather than random
// samples are the point: a learner who finishes every test in a topic has seen
// every question in it, which random practice can never promise.
type TopicUsecase struct {
	contentRepo  domain.ContentRepository
	practiceRepo domain.PracticeRepository
	dashRepo     domain.DashboardRepository
}

func NewTopicUsecase(
	contentRepo domain.ContentRepository,
	practiceRepo domain.PracticeRepository,
	dashRepo domain.DashboardRepository,
) *TopicUsecase {
	return &TopicUsecase{contentRepo: contentRepo, practiceRepo: practiceRepo, dashRepo: dashRepo}
}

type TopicsResponse struct {
	PackID           string                `json:"packId"`
	QuestionsPerTest int                   `json:"questionsPerTest"`
	Items            []domain.TopicSummary `json:"items"`
}

type TopicTestsResponse struct {
	PackID           string             `json:"packId"`
	Category         string             `json:"category"`
	QuestionsPerTest int                `json:"questionsPerTest"`
	QuestionCount    int                `json:"questionCount"`
	Items            []domain.TopicTest `json:"items"`
}

// testCount is how many numbered tests a category of n questions holds. The
// last test is short rather than dropped: a 1307-question bank ends on a test
// of 7, and those 7 questions are as examinable as any other.
func testCount(questionCount int) int {
	if questionCount <= 0 {
		return 0
	}
	return (questionCount + domain.QuestionsPerTest - 1) / domain.QuestionsPerTest
}

func (u *TopicUsecase) Topics(ctx context.Context, userID int64, packID string) (*TopicsResponse, error) {
	pack, err := u.contentRepo.GetPackByID(ctx, packID)
	if err != nil || pack == nil {
		return nil, errors.New("question pack not found")
	}

	counts, err := u.contentRepo.CountQuestionsByCategory(ctx, packID)
	if err != nil {
		return nil, err
	}

	// Progress is best-effort: a learner with no history yet is the normal
	// case on this screen, and a stats query that fails should cost the
	// percentages, not the list of topics.
	completed, _ := u.practiceRepo.CountCompletedTestsByCategory(ctx, userID, packID)
	accuracyRows, _ := u.dashRepo.GetCategoryAccuracy(ctx, userID)
	accuracy := make(map[string]domain.WeakCategoryInfo, len(accuracyRows))
	for _, row := range accuracyRows {
		accuracy[row.Category] = row
	}

	// Driven by domain.Categories, not by what the pack happens to contain, so
	// a topic with no questions yet still appears — empty and honest rather
	// than silently missing.
	items := make([]domain.TopicSummary, 0, len(domain.Categories))
	for _, category := range domain.Categories {
		summary := domain.TopicSummary{
			Category:        category,
			QuestionCount:   counts[category],
			TestCount:       testCount(counts[category]),
			CompletedTests:  completed[category],
			AccuracyPercent: accuracy[category].AccuracyPercent,
			DueMistakes:     accuracy[category].DueMistakes,
		}
		// A pack can shrink between releases; a count of tests completed that
		// is higher than the tests that now exist would show as "9 of 7".
		if summary.CompletedTests > summary.TestCount {
			summary.CompletedTests = summary.TestCount
		}
		items = append(items, summary)
	}

	return &TopicsResponse{
		PackID:           packID,
		QuestionsPerTest: domain.QuestionsPerTest,
		Items:            items,
	}, nil
}

func (u *TopicUsecase) TopicTests(ctx context.Context, userID int64, packID, category string) (*TopicTestsResponse, error) {
	if !domain.IsKnownCategory(category) {
		return nil, errors.New("unknown category")
	}
	pack, err := u.contentRepo.GetPackByID(ctx, packID)
	if err != nil || pack == nil {
		return nil, errors.New("question pack not found")
	}

	counts, err := u.contentRepo.CountQuestionsByCategory(ctx, packID)
	if err != nil {
		return nil, err
	}
	total := counts[category]

	results, _ := u.practiceRepo.GetTopicTestResults(ctx, userID, packID, category)

	items := make([]domain.TopicTest, 0, testCount(total))
	for index := 1; index <= testCount(total); index++ {
		// Every test is 20 questions except the last, which holds the
		// remainder.
		size := domain.QuestionsPerTest
		if remaining := total - (index-1)*domain.QuestionsPerTest; remaining < size {
			size = remaining
		}
		test := domain.TopicTest{Index: index, QuestionCount: size}
		if result, ok := results[index]; ok {
			test.Attempts = result.Attempts
			test.BestCorrect = result.BestCorrect
			test.LastAttemptAt = result.LastAttemptAt
		}
		// QuestionCount always comes from the pack as it is now, never from
		// the stored attempt: a test the learner sat when it was 20 questions
		// long is 20 questions long today too, unless the pack itself changed.
		items = append(items, test)
	}

	return &TopicTestsResponse{
		PackID:           packID,
		Category:         category,
		QuestionsPerTest: domain.QuestionsPerTest,
		QuestionCount:    total,
		Items:            items,
	}, nil
}
