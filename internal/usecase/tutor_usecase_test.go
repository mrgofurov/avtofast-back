package usecase_test

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/avtofast/avtofast-back/internal/domain"
	"github.com/avtofast/avtofast-back/internal/repository/memory"
	"github.com/avtofast/avtofast-back/internal/usecase"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// fakeTutorModel records what it was asked and replies with a canned answer.
type fakeTutorModel struct {
	reply *domain.TutorModelAnswer
	calls []domain.TutorModelRequest
}

func (f *fakeTutorModel) Answer(_ context.Context, req domain.TutorModelRequest) (*domain.TutorModelAnswer, error) {
	f.calls = append(f.calls, req)
	return f.reply, nil
}

func seedTutorBank(t *testing.T) *memory.MemoryStore {
	mem := memory.New()
	ctx := context.Background()
	now := time.Now().UTC()
	require.NoError(t, mem.CreatePack(ctx, &domain.QuestionPack{
		PackID: "uz-theory-2026-09", Version: "2026.09.1", Title: "t",
		Status: "published", PublishedAt: &now,
	}))

	add := func(id, category, prompt, explanation string) {
		require.NoError(t, mem.CreateQuestion(ctx, &domain.Question{
			PublicID: id, PackID: "uz-theory-2026-09", ContentVersion: "2026.09.1",
			Category: category, CorrectChoiceID: "a", Status: "published",
			Translations: map[string]domain.QuestionTranslationData{
				domain.LocaleUzLatn: {
					Prompt: prompt,
					Choices: []domain.ChoiceItem{
						{ID: "a", Text: "To'g'ri javob " + id, Position: 1},
						{ID: "b", Text: "Noto'g'ri javob " + id, Position: 2},
					},
					Explanation: explanation,
				},
			},
		}))
	}
	add("pd_1", domain.CategoryIntersections, "Teng ahamiyatli chorrahada kim birinchi o'tadi?",
		"YHQ 16-bobi 105-bandiga asosan, teng ahamiyatga ega bo'lgan yo'llar kesishgan chorrahada relssiz transport vositasining haydovchisi o'ng tomondan yaqinlashayotgan transport vositalariga yo'l berishi shart.")
	add("pd_2", domain.CategoryTrafficRules, "Shatakka olishda qattiq ulagich bilan masofa qancha bo'lishi kerak?",
		"Yo‘l harakati qoidalarining 144 bandiga asosan, egiluvchan ulagich bilan shatakka olishda masofa 4 — 6 metr, qattiq ulagich bilan esa 4 metrdan ko‘p bo‘lmasligi kerak.")
	add("pd_3", domain.CategoryFirstAid, "Suyak singanda nima qilinadi?",
		"Suyaklar singanda shikastlangan qo'l yoki oyoqni har qanday qattiq vosita yordamida mustahkamlab, harakatsizlantirish kerak.")
	// Same explanation as pd_1, asked another way: must not be a second source.
	add("pd_4", domain.CategoryIntersections, "Chorrahada o'ng tomondan kelayotganga yo'l beriladimi?",
		"YHQ 16-bobi 105-bandiga asosan, teng ahamiyatga ega bo'lgan yo'llar kesishgan chorrahada relssiz transport vositasining haydovchisi o'ng tomondan yaqinlashayotgan transport vositalariga yo'l berishi shart.")
	return mem
}

func TestTutor_GroundedAnswerCitesRetrievedClause(t *testing.T) {
	mem := seedTutorBank(t)
	model := &fakeTutorModel{reply: &domain.TutorModelAnswer{
		Answer: "O'ng tomondan kelayotganga yo'l berasiz (YHQ 16-bobi 105-band).", Grounded: true, UsedSources: []int{1, 1, 9},
	}}
	tutor := usecase.NewTutorUsecase(mem, nil, model)

	res, err := tutor.Ask(context.Background(), usecase.TutorRequest{Message: "Teng ahamiyatli chorrahada kim birinchi o‘tadi?"})
	require.NoError(t, err)

	require.Len(t, model.calls, 1)
	sources := model.calls[0].Sources
	require.NotEmpty(t, sources)
	assert.Contains(t, []string{"pd_1", "pd_4"}, sources[0].QuestionID)
	for _, s := range sources {
		assert.NotEqual(t, "pd_3", s.QuestionID, "first aid has nothing to do with intersections")
	}
	explanations := map[string]bool{}
	for _, s := range sources {
		assert.False(t, explanations[s.Text], "duplicate explanation sent twice")
		explanations[s.Text] = true
	}

	assert.True(t, res.Grounded)
	// Repeated and out-of-range source numbers are dropped, not trusted.
	require.Len(t, res.Citations, 1)
	assert.Equal(t, "YHQ 16-bobi 105-bandiga", res.Citations[0].Clause)
	assert.Contains(t, res.Citations[0].Text, "o'ng tomondan")
}

func TestTutor_UngroundedAnswerIsReplacedByFixedText(t *testing.T) {
	mem := seedTutorBank(t)
	model := &fakeTutorModel{reply: &domain.TutorModelAnswer{
		Answer: "Jarima 500 ming so'm.", Grounded: false,
	}}
	tutor := usecase.NewTutorUsecase(mem, nil, model)

	res, err := tutor.Ask(context.Background(), usecase.TutorRequest{Message: "Chorrahada qizil chiroqqa o'tsam jarima qancha?"})
	require.NoError(t, err)
	assert.False(t, res.Grounded)
	assert.Equal(t, usecase.TutorNotFoundAnswer, res.Answer)
	assert.Empty(t, res.Citations)
}

func TestTutor_GroundedClaimWithoutValidSourcesIsNotTrusted(t *testing.T) {
	mem := seedTutorBank(t)
	model := &fakeTutorModel{reply: &domain.TutorModelAnswer{Answer: "Ha.", Grounded: true, UsedSources: []int{42}}}
	tutor := usecase.NewTutorUsecase(mem, nil, model)

	res, err := tutor.Ask(context.Background(), usecase.TutorRequest{Message: "Shatakka olishda masofa qancha?"})
	require.NoError(t, err)
	assert.False(t, res.Grounded)
	assert.Equal(t, usecase.TutorNotFoundAnswer, res.Answer)
}

func TestTutor_NothingRetrievedSkipsTheModel(t *testing.T) {
	mem := seedTutorBank(t)
	model := &fakeTutorModel{}
	tutor := usecase.NewTutorUsecase(mem, nil, model)

	res, err := tutor.Ask(context.Background(), usecase.TutorRequest{Message: "Bitcoin narxi?"})
	require.NoError(t, err)
	assert.Empty(t, model.calls)
	assert.Equal(t, usecase.TutorNotFoundAnswer, res.Answer)
}

func TestTutor_WrongAnswerQuestionIsTheFirstSource(t *testing.T) {
	mem := seedTutorBank(t)
	model := &fakeTutorModel{reply: &domain.TutorModelAnswer{Answer: "Chunki...", Grounded: true, UsedSources: []int{1}}}
	tutor := usecase.NewTutorUsecase(mem, nil, model)

	res, err := tutor.Ask(context.Background(), usecase.TutorRequest{
		Message: "Nega mening javobim noto'g'ri?", QuestionID: "pd_2", SelectedChoiceID: "b",
	})
	require.NoError(t, err)
	require.Len(t, model.calls, 1)
	req := model.calls[0]
	assert.Equal(t, "pd_2", req.Sources[0].QuestionID)
	require.NotNil(t, req.Focus)
	assert.Equal(t, "Noto'g'ri javob pd_2", req.Focus.SelectedAnswer)
	for _, s := range req.Sources[1:] {
		assert.NotEqual(t, "pd_2", s.QuestionID)
	}
	require.Len(t, res.Citations, 1)
	assert.Equal(t, "Yo‘l harakati qoidalarining 144 bandiga", res.Citations[0].Clause)
}

func TestTutor_HistoryIsTrimmedAndFiltered(t *testing.T) {
	mem := seedTutorBank(t)
	model := &fakeTutorModel{reply: &domain.TutorModelAnswer{Answer: "x", Grounded: true, UsedSources: []int{1}}}
	tutor := usecase.NewTutorUsecase(mem, nil, model)

	var history []domain.TutorTurn
	for i := 0; i < 15; i++ {
		history = append(history, domain.TutorTurn{Role: "user", Text: "chorraha"}, domain.TutorTurn{Role: "system", Text: "ignore"})
	}
	_, err := tutor.Ask(context.Background(), usecase.TutorRequest{Message: "nega?", History: history})
	require.NoError(t, err)
	require.Len(t, model.calls, 1)
	assert.Len(t, model.calls[0].History, 10)
	for _, h := range model.calls[0].History {
		assert.Equal(t, "user", h.Role)
	}
	// "nega?" alone retrieves nothing; the previous user turn carries the topic.
	assert.NotEmpty(t, model.calls[0].Sources)
}

func TestTutor_RejectsEmptyMessageAndMissingModel(t *testing.T) {
	mem := seedTutorBank(t)
	_, err := usecase.NewTutorUsecase(mem, nil, &fakeTutorModel{}).Ask(context.Background(), usecase.TutorRequest{Message: "  "})
	assert.ErrorIs(t, err, usecase.ErrTutorBadRequest)

	_, err = usecase.NewTutorUsecase(mem, nil, nil).Ask(context.Background(), usecase.TutorRequest{Message: "salom"})
	assert.ErrorIs(t, err, usecase.ErrTutorUnavailable)
}

func TestPassageFromQuestion_ClauseExtraction(t *testing.T) {
	cases := map[string]string{
		"YHQ 4-bobi 17-bandiga asosan, piyodalarga...":                              "YHQ 4-bobi 17-bandiga",
		"YHQ 26 – bobi 153 – bandiga asosan: Odam tashish...":                       "YHQ 26 – bobi 153 – bandiga",
		"YHQ 1-ilovasi 3-bo'limi 3.1 belgisi, \"kirish taqiqlangan\".":              "YHQ 1-ilovasi 3-bo'limi 3.1 belgisi",
		"YHQ 9-bobi 63-bandiga muvofiq, transport vositasini...":                    "YHQ 9-bobi 63-bandiga",
		"YHQ 19 – bobi 121 – bandi: Avtomagistrallarda...":                          "YHQ 19 – bobi 121 – bandi",
		"YHQ 1-ilovasining 1-bo'limidagi 1.3.1 belgi \"Bir izli temir yo'l\" – sh":  "YHQ 1-ilovasining 1-bo'limidagi 1.3.1 belgi",
		"YHQ 1-ilovasining 1-bo'limi 1.31.3. “Burilishning yo'nalishi” belgisi":     "YHQ 1-ilovasining 1-bo'limi 1.31.3",
		"Suyaklar singanda shikastlangan qo'l yoki oyoqni mustahkamlash kerak.":     "",
		"Egri yo‘l (burulish) da harakatlanayotgan avtomobilda markazdan qochirma.": "",
	}
	for explanation, want := range cases {
		q := &domain.Question{PublicID: "x", CorrectChoiceID: "a", Translations: map[string]domain.QuestionTranslationData{
			domain.LocaleUzLatn: {Prompt: "p", Explanation: explanation},
		}}
		p, ok := usecase.PassageFromQuestion(q, domain.LocaleUzLatn)
		require.True(t, ok)
		assert.Equal(t, want, p.Clause, explanation)
	}

	_, ok := usecase.PassageFromQuestion(&domain.Question{Translations: map[string]domain.QuestionTranslationData{
		domain.LocaleUzLatn: {Prompt: "p", Explanation: "  "},
	}}, domain.LocaleUzLatn)
	assert.False(t, ok, "a question without an explanation is not a source")
}

type fakeLegal struct{ paragraphs []domain.LegalParagraph }

func (f fakeLegal) ListParagraphs(context.Context) ([]domain.LegalParagraph, error) {
	return f.paragraphs, nil
}
func (f fakeLegal) ReplaceDocument(context.Context, string, []domain.LegalParagraph) error {
	return nil
}

func yhqParagraphs() []domain.LegalParagraph {
	ch16 := "16-bob. Tartibga solinmagan chorrahalar"
	signs := "1-ilova. Yoʻl belgilari"
	p := func(id, chapter, appendix, section, clause, text string) domain.LegalParagraph {
		return domain.LegalParagraph{Document: domain.DocumentRules, DocumentTitle: "YHQ", SourceElementID: id, SourceURL: "https://lex.uz/docs/-5953883",
			Chapter: chapter, Appendix: appendix, Section: section, ClauseNumber: clause, Text: text}
	}
	return []domain.LegalParagraph{
		p("-1", ch16, "", "", "105", "105. Teng ahamiyatga ega boʻlgan yoʻllar kesishgan chorrahada relssiz transport vositasining haydovchisi oʻngdan yaqinlashayotgan transport vositalariga yoʻl berishi shart."),
		p("-2", ch16, "", "", "105", "Bunday chorrahalarda tramvay relssiz transport vositalariga nisbatan oldin oʻtish huquqiga ega."),
		p("-3", ch16, "", "", "106", "106. Asosiy yoʻlning yoʻnalishi chorrahada oʻzgarganda..."),
		p("-4", "", signs, "2. Imtiyoz belgilari", "", "Imtiyoz belgilari harakatlanish navbatini belgilaydi."),
		p("-5", "", signs, "2. Imtiyoz belgilari", "2.4", "2.4. “Yoʻl bering!”. Haydovchi kesib oʻtilayotgan yoʻldan harakatlanayotgan transport vositalariga yoʻl berishi kerak."),
		p("-6", "", "", "", "", "orphan"),
	}
}

func TestLegalPassages_GroupsClausesAndLabelsThem(t *testing.T) {
	ps := usecase.LegalPassages(yhqParagraphs()[:5])
	require.Len(t, ps, 4)

	assert.Equal(t, "yhq:-1", ps[0].ID)
	assert.Equal(t, domain.TutorSourceRule, ps[0].Kind)
	assert.Equal(t, "YHQ 16-bob. Tartibga solinmagan chorrahalar › 105-band", ps[0].Clause)
	assert.Equal(t, yhqParagraphs()[0].Text+"\n"+yhqParagraphs()[1].Text, ps[0].Text)
	assert.Equal(t, "https://lex.uz/docs/-5953883#-1", ps[0].URL)

	assert.Equal(t, "YHQ 1-ilova. Yoʻl belgilari › 2. Imtiyoz belgilari", ps[2].Clause)
	assert.Equal(t, "YHQ 1-ilova. Yoʻl belgilari › 2. Imtiyoz belgilari › 2.4", ps[3].Clause)
}

func TestLegalPassages_SplitsLongClausesKeepingTheLead(t *testing.T) {
	var paragraphs []domain.LegalParagraph
	paragraphs = append(paragraphs, domain.LegalParagraph{Document: domain.DocumentRules, SourceElementID: "-0", Chapter: "1-bob. Umumiy qoidalar", ClauseNumber: "3",
		Text: "3. Ushbu Qoidalarda quyidagi asosiy tushunchalar qoʻllaniladi:"})
	for i := 1; i <= 20; i++ {
		paragraphs = append(paragraphs, domain.LegalParagraph{Document: domain.DocumentRules, SourceElementID: fmt.Sprintf("-%d", i), Chapter: "1-bob. Umumiy qoidalar", ClauseNumber: "3",
			Text: fmt.Sprintf("tushuncha %d — %s", i, strings.Repeat("izoh ", 30))})
	}
	ps := usecase.LegalPassages(paragraphs)
	require.Greater(t, len(ps), 2)
	for i, p := range ps {
		assert.True(t, strings.HasPrefix(p.Text, "3. Ushbu Qoidalarda"), "part %d lost its lead", i)
		assert.LessOrEqual(t, len([]rune(p.Text)), 1500)
		assert.Equal(t, "YHQ 1-bob. Umumiy qoidalar › 3-band", p.Clause)
	}
	assert.True(t, strings.Contains(ps[1].Text, "\n…\n"), "a later part marks the gap after the lead")
	// Every paragraph ends up in exactly one part.
	all := strings.Join(func() []string {
		var t []string
		for _, p := range ps {
			t = append(t, p.Text)
		}
		return t
	}(), "\n")
	for _, p := range paragraphs[1:] {
		assert.Equal(t, 1, strings.Count(all, p.Text))
	}
}

func TestTutor_RulesAreSourcesAndCitedWithTheirClause(t *testing.T) {
	mem := seedTutorBank(t)
	model := &fakeTutorModel{reply: &domain.TutorModelAnswer{Answer: "O'ngdagiga yo'l berasiz.", Grounded: true, UsedSources: []int{1, 6}}}
	tutor := usecase.NewTutorUsecase(mem, fakeLegal{yhqParagraphs()}, model)

	res, err := tutor.Ask(context.Background(), usecase.TutorRequest{Message: "Teng ahamiyatli chorrahada kim birinchi o'tadi?"})
	require.NoError(t, err)
	require.Len(t, model.calls, 1)
	sources := model.calls[0].Sources
	require.NotEmpty(t, sources)
	assert.Equal(t, domain.TutorSourceRule, sources[0].Kind, "the Rules' own text ranks first")
	assert.Equal(t, "yhq:-1", sources[0].ID)

	kinds := map[string]int{}
	for _, s := range sources {
		kinds[s.Kind]++
	}
	assert.Positive(t, kinds[domain.TutorSourceQuestion], "bank explanations still come along")

	require.NotEmpty(t, res.Citations)
	c := res.Citations[0]
	assert.Equal(t, "rule", c.Kind)
	assert.Equal(t, "YHQ 16-bob. Tartibga solinmagan chorrahalar › 105-band", c.Clause)
	assert.Equal(t, "https://lex.uz/docs/-5953883#-1", c.URL)
	assert.Empty(t, c.QuestionID)
}

func TestTutor_WrongQuestionPullsInTheClauseItCites(t *testing.T) {
	mem := seedTutorBank(t)
	model := &fakeTutorModel{reply: &domain.TutorModelAnswer{Answer: "x", Grounded: true, UsedSources: []int{1}}}
	tutor := usecase.NewTutorUsecase(mem, fakeLegal{yhqParagraphs()}, model)

	_, err := tutor.Ask(context.Background(), usecase.TutorRequest{Message: "Nega?", QuestionID: "pd_1", SelectedChoiceID: "b"})
	require.NoError(t, err)
	sources := model.calls[0].Sources
	assert.Equal(t, "pd_1", sources[0].ID)
	assert.Equal(t, "yhq:-1", sources[1].ID, "clause 105, which pd_1's explanation cites, is the top rule")
}

func TestLegalPassages_ArticlesAreCitedByDocumentAndHeading(t *testing.T) {
	heading := "128³-modda. Transport vositalari haydovchilarining belgilangan harakat tezligini oshirib yuborishi"
	p := func(id, text string) domain.LegalParagraph {
		return domain.LegalParagraph{Document: domain.DocumentCode, DocumentTitle: "MJtK", SourceElementID: id,
			SourceURL: "https://lex.uz/docs/-97664", Chapter: "XI bob. TRANSPORTDAGI ...", Section: heading, ClauseNumber: "128³", Text: text}
	}
	ps := usecase.LegalPassages([]domain.LegalParagraph{
		p("-1", heading),
		p("-2", "Tezlikni soatiga 20 kilometrdan koʻp boʻlmagan kattalikda oshirib yuborish —"),
		p("-3", "bazaviy hisoblash miqdorining bir baravari miqdorida jarima solishga sabab boʻladi."),
	})
	require.Len(t, ps, 1)
	assert.Equal(t, "mjtk:-1", ps[0].ID)
	assert.Equal(t, "MJtK › "+heading, ps[0].Clause)
	assert.Equal(t, "https://lex.uz/docs/-97664#-1", ps[0].URL)
	assert.True(t, strings.HasPrefix(ps[0].Text, heading+"\n"))
}
