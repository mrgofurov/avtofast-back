package usecase

import (
	"context"
	"errors"
	"strings"
	"sync"
	"time"

	"github.com/avtofast/avtofast-back/internal/domain"
)

var (
	ErrTutorUnavailable = errors.New("tutor model is not configured")
	ErrTutorBadRequest  = errors.New("invalid tutor request")
)

const (
	tutorMaxMessageRunes = 1000
	tutorMaxHistoryTurns = 10
	tutorMaxTurnRunes    = 2000
	// Official text leads; bank explanations add how the exam asks about
	// it. A learner's wrong question comes on top as source 1.
	tutorRuleSources     = 6
	tutorQuestionSources = 3
	tutorCorpusTTL       = 15 * time.Minute
)

// TutorNotFoundAnswer is what the learner reads whenever the tutor cannot
// ground an answer in the official sources. It is fixed text rather than
// whatever the model wrote, so an ungrounded reply can never carry a rule.
const TutorNotFoundAnswer = "Bu savolga rasmiy manbalarimda (Yo'l harakati qoidalari, Yo'l harakati to'g'risidagi qonun, Ma'muriy javobgarlik kodeksi va savollar bazasining rasmiy izohlari) javob topilmadi, shuning uchun taxmin qilmayman. Savolni boshqacha ifodalab ko'ring yoki aniq bir qoida, belgi yoki holat haqida so'rang."

type TutorRequest struct {
	Message string `json:"message"`
	// QuestionID is a question the learner just got wrong; its explanation is
	// always among the sources. SelectedChoiceID is what they picked.
	QuestionID       string             `json:"questionId,omitempty"`
	SelectedChoiceID string             `json:"selectedChoiceId,omitempty"`
	History          []domain.TutorTurn `json:"history,omitempty"`
}

type TutorCitation struct {
	ID   string `json:"id"`
	Kind string `json:"kind"` // "rule" or "question"
	// QuestionID is set for question sources.
	QuestionID string `json:"questionId,omitempty"`
	// Clause names the source: chapter or appendix and clause number for a
	// rule, the explanation's own reference for a question (may be empty).
	Clause string `json:"clause,omitempty"`
	// Text is the official wording, verbatim.
	Text string `json:"text"`
	// URL links a rule to its paragraph on lex.uz.
	URL string `json:"url,omitempty"`
}

type TutorResponse struct {
	Answer    string          `json:"answer"`
	Grounded  bool            `json:"grounded"`
	Citations []TutorCitation `json:"citations"`
}

type TutorUsecase struct {
	content domain.ContentRepository
	legal   domain.LegalRepository // nil: answer from the question bank alone
	model   domain.TutorModel
	locale  string

	mu      sync.Mutex
	corp    *tutorCorpus
	builtAt time.Time
}

// tutorCorpus is everything the tutor may answer from, in two indexes so the
// many short official clauses and articles (the Rules, the traffic law, the
// traffic chapter of the administrative code) and the fewer, longer bank
// explanations are each ranked against their own kind.
type tutorCorpus struct {
	rules     *tutorIndex
	questions *tutorIndex
}

func NewTutorUsecase(content domain.ContentRepository, legal domain.LegalRepository, model domain.TutorModel) *TutorUsecase {
	return &TutorUsecase{content: content, legal: legal, model: model, locale: domain.LocaleUzLatn}
}

// corpus returns the retrieval indexes, rebuilt from the database when older
// than the TTL, so a re-import of the Rules or a newly published pack shows
// up without a restart. A failed rebuild keeps serving the old corpus.
func (u *TutorUsecase) corpus(ctx context.Context) (*tutorCorpus, error) {
	u.mu.Lock()
	defer u.mu.Unlock()
	if u.corp != nil && time.Since(u.builtAt) < tutorCorpusTTL {
		return u.corp, nil
	}

	c, err := u.buildCorpus(ctx)
	if err != nil {
		if u.corp != nil {
			return u.corp, nil
		}
		return nil, err
	}
	u.corp, u.builtAt = c, time.Now()
	return c, nil
}

func (u *TutorUsecase) buildCorpus(ctx context.Context) (*tutorCorpus, error) {
	questions, err := u.questionPassages(ctx)
	if err != nil {
		return nil, err
	}
	var rules []domain.TutorPassage
	if u.legal != nil {
		paragraphs, err := u.legal.ListParagraphs(ctx)
		if err != nil {
			return nil, err
		}
		rules = LegalPassages(paragraphs)
	}
	return &tutorCorpus{rules: newTutorIndex(rules), questions: newTutorIndex(questions)}, nil
}

func (u *TutorUsecase) questionPassages(ctx context.Context) ([]domain.TutorPassage, error) {
	pack, err := u.content.GetActivePack(ctx)
	if err != nil {
		return nil, err
	}
	if pack == nil {
		return nil, nil
	}

	var passages []domain.TutorPassage
	cursor := ""
	for {
		page, next, err := u.content.GetQuestionsByPack(ctx, pack.PackID, "", cursor, 100)
		if err != nil {
			return nil, err
		}
		for _, q := range page {
			if p, ok := PassageFromQuestion(q, u.locale); ok {
				passages = append(passages, p)
			}
		}
		if next == "" || next == cursor || len(page) == 0 {
			break
		}
		cursor = next
	}
	return passages, nil
}

func truncateRunes(s string, n int) string {
	if r := []rune(s); len(r) > n {
		return string(r[:n])
	}
	return s
}

func (u *TutorUsecase) Ask(ctx context.Context, req TutorRequest) (*TutorResponse, error) {
	if u.model == nil {
		return nil, ErrTutorUnavailable
	}
	message := strings.TrimSpace(req.Message)
	if message == "" {
		return nil, ErrTutorBadRequest
	}
	message = truncateRunes(message, tutorMaxMessageRunes)

	var history []domain.TutorTurn
	for _, t := range req.History {
		text := strings.TrimSpace(t.Text)
		if text == "" || (t.Role != "user" && t.Role != "assistant") {
			continue
		}
		history = append(history, domain.TutorTurn{Role: t.Role, Text: truncateRunes(text, tutorMaxTurnRunes)})
	}
	if len(history) > tutorMaxHistoryTurns {
		history = history[len(history)-tutorMaxHistoryTurns:]
	}

	corp, err := u.corpus(ctx)
	if err != nil {
		return nil, err
	}

	var sources []domain.TutorPassage
	var focus *domain.TutorFocus
	query := message
	if req.QuestionID != "" {
		q, err := u.content.GetQuestionByID(ctx, req.QuestionID)
		if err != nil {
			return nil, err
		}
		if q != nil {
			if p, ok := PassageFromQuestion(q, u.locale); ok {
				sources = append(sources, p)
				// The explanation names the clause ("16-bobi 105-bandiga"),
				// which is what finds that clause's own text.
				query += " " + p.Prompt + " " + p.Answer + " " + p.Text
				focus = &domain.TutorFocus{}
				for _, c := range q.Translations[u.locale].Choices {
					if c.ID == req.SelectedChoiceID {
						focus.SelectedAnswer = strings.TrimSpace(c.Text)
					}
				}
			}
		}
	}
	// A follow-up ("nega?", "batafsilroq") says little on its own; the
	// learner's previous question carries the topic.
	for i := len(history) - 1; i >= 0; i-- {
		if history[i].Role == "user" {
			query += " " + history[i].Text
			break
		}
	}

	exclude := ""
	if len(sources) > 0 {
		exclude = sources[0].ID
	}
	sources = append(sources, corp.rules.search(query, tutorRuleSources, "")...)
	sources = append(sources, corp.questions.search(query, tutorQuestionSources, exclude)...)

	// Nothing in the Rules or the bank shares a word with the question: there
	// is nothing to answer from, and asking the model would only invite a guess.
	if len(sources) == 0 {
		return &TutorResponse{Answer: TutorNotFoundAnswer, Citations: []TutorCitation{}}, nil
	}

	ans, err := u.model.Answer(ctx, domain.TutorModelRequest{
		Message: message,
		History: history,
		Sources: sources,
		Focus:   focus,
	})
	if err != nil {
		return nil, err
	}

	// Citations come from the sources the model named, never from its own
	// text, so every clause shown to the learner is one that really exists.
	citations := []TutorCitation{}
	seen := make(map[int]bool)
	for _, n := range ans.UsedSources {
		if n < 1 || n > len(sources) || seen[n] {
			continue
		}
		seen[n] = true
		s := sources[n-1]
		citations = append(citations, TutorCitation{
			ID: s.ID, Kind: s.Kind, QuestionID: s.QuestionID, Clause: s.Clause, Text: s.Text, URL: s.URL,
		})
	}

	answer := strings.TrimSpace(ans.Answer)
	if !ans.Grounded || len(citations) == 0 || answer == "" {
		return &TutorResponse{Answer: TutorNotFoundAnswer, Citations: []TutorCitation{}}, nil
	}
	return &TutorResponse{Answer: answer, Grounded: true, Citations: citations}, nil
}
