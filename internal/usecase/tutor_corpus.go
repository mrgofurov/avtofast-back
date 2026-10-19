package usecase

import (
	"math"
	"regexp"
	"sort"
	"strings"
	"unicode"

	"github.com/avtofast/avtofast-back/internal/domain"
)

// The tutor's corpus is the question bank itself: every published question
// with its official explanation. Retrieval is BM25 over that text, which is
// small enough (hundreds to a few thousand passages) to hold in memory and
// rebuild from the database whenever content may have changed.

const (
	bm25K1 = 1.2
	bm25B  = 0.75
	// Uzbek is agglutinative: "chorrahada", "chorrahaga" and "chorrahalarda"
	// are one word to a learner. Cutting every word to its first few letters
	// is a crude stemmer, but it matches those forms and costs nothing.
	stemRunes = 6
)

var tutorStopwords = map[string]bool{
	"va": true, "yoki": true, "bilan": true, "uchun": true, "bu": true, "shu": true,
	"u": true, "bir": true, "ham": true, "esa": true, "emas": true, "mi": true,
	"qaysi": true, "qanday": true, "qancha": true, "nechta": true, "qayerda": true, "nima": true, "nimani": true, "nimaga": true, "nega": true, "necha": true,
	"qachon": true, "kim": true, "agar": true, "kerak": true, "mumkin": true,
	"holda": true, "deb": true, "lekin": true, "ammo": true, "men": true,
	"menga": true, "siz": true, "biz": true, "edi": true, "ekan": true,
}

// normalizeUz folds the many apostrophes Uzbek Latin is typed with (o', o‘,
// oʻ, o`) so that "yo'l" written any way is the same word.
func normalizeUz(s string) string {
	var b strings.Builder
	b.Grow(len(s))
	for _, r := range strings.ToLower(s) {
		switch r {
		case '\'', '‘', '’', 'ʻ', 'ʼ', '`', '´', 'ʹ':
			continue
		}
		b.WriteRune(r)
	}
	return b.String()
}

func tokenizeUz(s string) []string {
	fields := strings.FieldsFunc(normalizeUz(s), func(r rune) bool {
		return !unicode.IsLetter(r) && !unicode.IsDigit(r) && r != '.'
	})
	out := make([]string, 0, len(fields))
	for _, f := range fields {
		f = strings.Trim(f, ".")
		if f == "" || tutorStopwords[f] {
			continue
		}
		// Sign and clause numbers ("2.4", "5.16.1") stay whole; words are stemmed.
		if !unicode.IsDigit([]rune(f)[0]) {
			f = strings.ReplaceAll(f, ".", "")
			r := []rune(f)
			if len(r) > stemRunes {
				r = r[:stemRunes]
			}
			// A final k or q turns into g or gʻ before a suffix ("tezlik",
			// "tezligi"; "chiroq", "chirogʻi"), so the stem's last consonant
			// is folded to one letter.
			if last := len(r) - 1; last > 0 && (r[last] == 'g' || r[last] == 'q') {
				r[last] = 'k'
			}
			f = string(r)
		}
		if f == "" {
			continue
		}
		out = append(out, f)
	}
	return out
}

// clauseLead matches the reference an official explanation opens with, up to
// the word that ends it: "YHQ 4-bobi 17-bandiga asosan, ..." gives
// "YHQ 4-bobi 17-bandiga".
// A quoted sign name ends it too: `YHQ 1-ilovasi 1.3.1 belgi "Bir izli..."`.
var clauseLead = regexp.MustCompile(`(?i)^\s*((?:YHQ|Yo.{0,2}l harakati qoidalari)[^:,\n"“«]{0,120}?)\s*(?:\basosan\b|\bmuvofiq\b|\bbinoan\b|[:,\n"“«]|$)`)

// extractClause returns the clause an explanation cites, verbatim, or "" when
// it cites none (first-aid and vehicle-physics explanations mostly do not).
func extractClause(explanation string) string {
	m := clauseLead.FindStringSubmatch(explanation)
	if m == nil {
		return ""
	}
	clause := strings.TrimSpace(strings.TrimRight(m[1], " .;-–—"))
	if len([]rune(clause)) < 6 {
		return ""
	}
	return clause
}

// PassageFromQuestion turns a bank question into tutor source text. ok is
// false when the question has no explanation in that locale: without the
// official text there is nothing the tutor may quote.
func PassageFromQuestion(q *domain.Question, locale string) (domain.TutorPassage, bool) {
	tr, found := q.Translations[locale]
	explanation := strings.TrimSpace(tr.Explanation)
	if !found || explanation == "" {
		return domain.TutorPassage{}, false
	}
	answer := ""
	for _, c := range tr.Choices {
		if c.ID == q.CorrectChoiceID {
			answer = strings.TrimSpace(c.Text)
		}
	}
	return domain.TutorPassage{
		ID:         q.PublicID,
		Kind:       domain.TutorSourceQuestion,
		QuestionID: q.PublicID,
		Category:   q.Category,
		Prompt:     strings.TrimSpace(tr.Prompt),
		Answer:     answer,
		Text:       explanation,
		Clause:     extractClause(explanation),
	}, true
}

const (
	// A clause longer than this is split, so one long list (the definitions
	// in clause 3 run to dozens of paragraphs) does not become one source
	// that drowns the others in the prompt.
	rulePassageMaxRunes = 1500
	ruleChunkRunes      = 1000
	chapterLabelRunes   = 60
	articleLabelRunes   = 120
)

func shorten(s string, n int) string {
	if r := []rune(s); len(r) > n {
		return strings.TrimSpace(string(r[:n])) + "…"
	}
	return s
}

// legalLabel is how an official text is cited:
// "YHQ 16-bob. Tartibga solinmagan chorrahalar › 105-band",
// "YHQ 1-ilova. Yoʻl belgilari › 2. Imtiyoz belgilari › 2.4" or
// "MJtK › 128³-modda. Transport vositalari haydovchilarining ...".
func legalLabel(p domain.LegalParagraph) string {
	if p.Document == domain.DocumentLaw || p.Document == domain.DocumentCode {
		if p.Section == "" {
			return p.DocumentTitle + " › " + shorten(p.Chapter, chapterLabelRunes)
		}
		return p.DocumentTitle + " › " + shorten(p.Section, articleLabelRunes)
	}
	parts := []string{}
	if p.Appendix == "" {
		parts = append(parts, shorten(p.Chapter, chapterLabelRunes))
		if p.ClauseNumber != "" {
			parts = append(parts, p.ClauseNumber+"-band")
		}
	} else {
		parts = append(parts, p.Appendix)
		if p.Section != "" {
			parts = append(parts, strings.TrimRight(p.Section, ":."))
		}
		if p.ClauseNumber != "" {
			parts = append(parts, p.ClauseNumber)
		}
	}
	return "YHQ " + strings.Join(parts, " › ")
}

// LegalPassages groups official paragraphs into citable passages: one per
// clause (a sign, a marking, a numbered band), per law or code article, or
// per section introduction. Paragraph text is kept verbatim; a clause split
// for length repeats its opening paragraph (for an article, its heading) in
// every part, so each part still says what it is about.
func LegalPassages(paragraphs []domain.LegalParagraph) []domain.TutorPassage {
	var out []domain.TutorPassage
	flush := func(group []domain.LegalParagraph) {
		if len(group) == 0 {
			return
		}
		label := legalLabel(group[0])
		total := 0
		for _, p := range group {
			total += len([]rune(p.Text))
		}
		emit := func(first domain.LegalParagraph, texts []string) {
			out = append(out, domain.TutorPassage{
				ID:     first.Document + ":" + first.SourceElementID,
				Kind:   domain.TutorSourceRule,
				Text:   strings.Join(texts, "\n"),
				Clause: label,
				URL:    first.SourceURL + "#" + first.SourceElementID,
			})
		}
		if total <= rulePassageMaxRunes {
			texts := make([]string, len(group))
			for i, p := range group {
				texts[i] = p.Text
			}
			emit(group[0], texts)
			return
		}
		lead := group[0].Text
		texts, size, start := []string{lead}, len([]rune(lead)), group[0]
		for _, p := range group[1:] {
			n := len([]rune(p.Text))
			if size+n > ruleChunkRunes && len(texts) > 1 {
				emit(start, texts)
				texts, size, start = []string{lead, "…"}, len([]rune(lead)), p
			}
			texts = append(texts, p.Text)
			size += n
		}
		emit(start, texts)
	}

	var group []domain.LegalParagraph
	for _, p := range paragraphs {
		if len(group) > 0 {
			g := group[0]
			if g.Document != p.Document || g.Chapter != p.Chapter || g.Appendix != p.Appendix || g.Section != p.Section || g.ClauseNumber != p.ClauseNumber {
				flush(group)
				group = nil
			}
		}
		group = append(group, p)
	}
	flush(group)
	return out
}

type tutorDoc struct {
	passage domain.TutorPassage
	tf      map[string]int
	length  int
}

type tutorIndex struct {
	docs   []tutorDoc
	byID   map[string]int
	df     map[string]int
	avgLen float64
}

func newTutorIndex(passages []domain.TutorPassage) *tutorIndex {
	idx := &tutorIndex{byID: make(map[string]int), df: make(map[string]int)}
	// Several questions share one explanation word for word (the same clause
	// asked four ways). As sources they would say the same thing four times
	// and crowd out other clauses, so only the first is indexed.
	seen := make(map[string]bool)
	total := 0
	for _, p := range passages {
		if seen[p.Text] {
			continue
		}
		seen[p.Text] = true

		tokens := tokenizeUz(p.Clause + " " + p.Prompt + " " + p.Answer + " " + p.Text)
		tf := make(map[string]int, len(tokens))
		for _, t := range tokens {
			tf[t]++
		}
		for t := range tf {
			idx.df[t]++
		}
		idx.byID[p.ID] = len(idx.docs)
		idx.docs = append(idx.docs, tutorDoc{passage: p, tf: tf, length: len(tokens)})
		total += len(tokens)
	}
	if len(idx.docs) > 0 {
		idx.avgLen = float64(total) / float64(len(idx.docs))
	}
	return idx
}

// learnerWords maps how learners say things to how the law does, so a
// question in everyday words still finds the article. It only widens what
// retrieval looks for; the answer is still built from the sources' own text.
var learnerWords = map[string]string{
	"qizil":  "taqiqlovchi svetofor", // "qizil chiroqqa o'tsam" → svetoforning taqiqlovchi signali
	"jazo":   "jarima javobgarlik",
	"shtraf": "jarima",
	"prava":  "guvohnoma",
}

// queryConcepts turns a query into concepts, each a set of index terms that
// count as the same word: the term itself, longer index terms it is the start
// of ("mast" finds "mastlik"), and the legal wording from learnerWords.
func (idx *tutorIndex) queryConcepts(query string) [][]string {
	tokens := tokenizeUz(query)
	for _, t := range tokens {
		if extra, ok := learnerWords[t]; ok {
			tokens = append(tokens, tokenizeUz(extra)...)
		}
	}
	seen := map[string]bool{}
	var concepts [][]string
	for _, t := range tokens {
		if seen[t] {
			continue
		}
		seen[t] = true
		terms := []string{}
		if idx.df[t] > 0 {
			terms = append(terms, t)
		}
		// Short words only: a full six-letter stem already matches its forms.
		if n := len([]rune(t)); n >= 4 && n < stemRunes && !unicode.IsDigit([]rune(t)[0]) {
			for v := range idx.df {
				if v != t && strings.HasPrefix(v, t) {
					terms = append(terms, v)
				}
			}
		}
		if len(terms) > 0 {
			concepts = append(concepts, terms)
		}
	}
	return concepts
}

// search returns up to k passages relevant to query, best first. Passages
// scoring far below the best one are dropped: a weak tail is noise the model
// would have to argue its way past.
func (idx *tutorIndex) search(query string, k int, exclude string) []domain.TutorPassage {
	if idx == nil || len(idx.docs) == 0 {
		return nil
	}
	concepts := idx.queryConcepts(query)
	if len(concepts) == 0 {
		return nil
	}

	n := float64(len(idx.docs))
	type scored struct {
		i     int
		score float64
	}
	var hits []scored
	for i, d := range idx.docs {
		if d.passage.ID == exclude {
			continue
		}
		score := 0.0
		for _, terms := range concepts {
			// A concept counts once, by its best-matching term, however many
			// of its forms the passage uses.
			best := 0.0
			for _, t := range terms {
				f := float64(d.tf[t])
				if f == 0 {
					continue
				}
				df := float64(idx.df[t])
				idf := math.Log(1 + (n-df+0.5)/(df+0.5))
				best = math.Max(best, idf*f*(bm25K1+1)/(f+bm25K1*(1-bm25B+bm25B*float64(d.length)/idx.avgLen)))
			}
			score += best
		}
		if score > 0 {
			hits = append(hits, scored{i, score})
		}
	}
	sort.Slice(hits, func(a, b int) bool { return hits[a].score > hits[b].score })

	var out []domain.TutorPassage
	for _, h := range hits {
		if len(out) == k || h.score < hits[0].score*0.35 {
			break
		}
		out = append(out, idx.docs[h.i].passage)
	}
	return out
}
