package usecase

import (
	"regexp"
	"strconv"
	"strings"

	"github.com/avtofast/avtofast-back/internal/domain"
)

// TopicClassifier files a question under a topic by the clause its official
// explanation cites: "YHQ 16-bobi 105-bandiga" is chapter 16, band 105;
// "YHQ 1-ilovasining 3-bo'limi 3.24 belgisiga" is the prohibitory signs;
// "Yo'l harakati qoidalarining 144 bandiga" names only a band, and the Rules'
// own text says band 144 is in chapter 24. The explanation is the official
// statement of what the question is about, so this is a lookup, not a guess.
//
// Explanations that cite no clause (first aid, vehicle physics) are filed by
// their wording, and that is reported as such so it can be checked.
type TopicClassifier struct {
	// bandChapter maps a band of the Rules' main text ("105", "115¹") to its
	// chapter number ("16", "17¹").
	bandChapter map[string]string
	// rules are the Rules' clauses (first paragraph and pooled words), for
	// explanations that quote the Rules without citing them.
	rules []ruleWords
}

type ruleWords struct {
	paragraph domain.LegalParagraph
	words     map[string]bool
}

// NewTopicClassifier builds the band index from the Rules' paragraphs (as
// stored by cmd/lex-import). Without them, explanations that cite a band but
// no chapter cannot be placed and fall back to their wording.
func NewTopicClassifier(rules []domain.LegalParagraph) *TopicClassifier {
	c := &TopicClassifier{bandChapter: map[string]string{}}
	// A clause is often several paragraphs (a lead and its list), and an
	// explanation quotes it across them, so words are pooled per clause.
	clause := map[string]int{}
	for _, p := range rules {
		if p.Document != domain.DocumentRules {
			continue
		}
		key := p.Chapter + "|" + p.Appendix + "|" + p.Section + "|" + p.ClauseNumber
		i, ok := clause[key]
		if !ok {
			i = len(c.rules)
			clause[key] = i
			c.rules = append(c.rules, ruleWords{paragraph: p, words: map[string]bool{}})
		}
		for _, t := range tokenizeUz(p.Text) {
			c.rules[i].words[t] = true
		}
		if p.Appendix != "" || p.ClauseNumber == "" {
			continue
		}
		if m := chapterNumberRe.FindStringSubmatch(p.Chapter); m != nil {
			c.bandChapter[p.ClauseNumber] = m[1]
		}
	}
	return c
}

// How a question was filed, for the classifier's report.
const (
	TopicByAppendix = "appendix" // "1-ilova", "2-ilova" ... with a section or sign number
	TopicByChapter  = "chapter"  // "16-bob"
	TopicByBand     = "band"     // a band alone, placed through the Rules' text
	TopicBySign     = "sign"     // a sign or marking number with no appendix named
	TopicByQuote    = "quote"    // no clause cited, but the Rules' words quoted
	TopicByWording  = "wording"  // no clause cited
)

var (
	chapterNumberRe = regexp.MustCompile(`^(\d+[` + "⁰¹²³⁴⁵⁶⁷⁸⁹" + `]*)\s*-bob`)

	refAppendixRe = regexp.MustCompile(`(\d)\s*-?\s*ilova`)
	refSectionRe  = regexp.MustCompile(`(\d+)\s*-?\s*bo'?lim`)
	refChapterRe  = regexp.MustCompile(`(\d+)\s*-?\s*bob`)
	// "105-bandiga", "144 bandiga", "100 - va 101 - bandlariga",
	// "106-,105 - va 104 - bandlariga": the first band of the list.
	refBandRe = regexp.MustCompile(`(\d+)\s*-?\s*(?:,\s*\d+\s*-?\s*)*(?:va\s+\d+\s*-?\s*)?band`)
	// "3.24", "1.14.1": a sign or marking number.
	refSignRe = regexp.MustCompile(`(?:^|[^\d.])(\d)\.\d+(?:\.\d+)*`)

	firstAidWords = []string{"jarohat", "shikast", "qon ", "qon-", "qonash", "singan", "suyak", "nafas", "yurak", "hush", "bog'lam", "bint", "tibbiy yordam", "birinchi yordam", "kuyish", "zaharlan", "umurtqa"}
	liabilityWords = []string{"jarima", "javobgarlik", "bazaviy hisoblash", "mjtk", "kodeks"}
)

// normalizeRef folds the apostrophes and dashes explanations are typed with,
// so the reference patterns need only one spelling.
var refFolder = strings.NewReplacer("‘", "'", "’", "'", "ʻ", "'", "ʼ", "'", "`", "'", "–", "-", "—", "-")

func normalizeRef(s string) string {
	return strings.ToLower(refFolder.Replace(s))
}

// Classify returns the topic for a question and how it was decided.
func (c *TopicClassifier) Classify(prompt, explanation string) (topic, how string) {
	text := normalizeRef(explanation)

	// The earliest reference is the one the explanation rests on; later ones
	// are usually cross-references ("... 7.18 belgisi bilan ...").
	first := func(re *regexp.Regexp) (int, []string) {
		loc := re.FindStringSubmatchIndex(text)
		if loc == nil {
			return -1, nil
		}
		m := make([]string, len(loc)/2)
		for i := range m {
			if loc[2*i] >= 0 {
				m[i] = text[loc[2*i]:loc[2*i+1]]
			}
		}
		return loc[0], m
	}
	aPos, aM := first(refAppendixRe)
	cPos, cM := first(refChapterRe)
	bPos, bM := first(refBandRe)

	earliest := func(pos int, others ...int) bool {
		if pos < 0 {
			return false
		}
		for _, o := range others {
			if o >= 0 && o < pos {
				return false
			}
		}
		return true
	}

	switch {
	case earliest(aPos, cPos, bPos):
		if t := c.appendixTopic(aM[1], text[aPos:]); t != "" {
			return t, TopicByAppendix
		}
	case earliest(cPos, aPos, bPos):
		band := ""
		if _, m := firstFrom(refBandRe, text, cPos); m != nil {
			band = m[1]
		}
		if t := c.chapterTopic(cM[1], band, text); t != "" {
			return t, TopicByChapter
		}
	case earliest(bPos, aPos, cPos):
		if chapter, ok := c.bandChapter[bM[1]]; ok {
			if t := c.chapterTopic(chapter, bM[1], text); t != "" {
				return t, TopicByBand
			}
		}
	}

	// No clause the Rules can place. A sign or marking number still says
	// which section it belongs to.
	if _, m := firstFrom(refSignRe, text, 0); m != nil && strings.Contains(text, "belgi") {
		if t := sectionTopic(1, m[1]); t != "" {
			return t, TopicBySign
		}
	}
	if _, m := firstFrom(refSignRe, text, 0); m != nil && strings.Contains(text, "chiziq") {
		if t := sectionTopic(2, m[1]); t != "" {
			return t, TopicBySign
		}
	}

	if t := c.quotedTopic(explanation); t != "" {
		return t, TopicByQuote
	}

	all := normalizeRef(prompt) + " " + text
	for _, w := range firstAidWords {
		if strings.Contains(all, w) {
			return domain.TopicFirstAid, TopicByWording
		}
	}
	for _, w := range liabilityWords {
		if strings.Contains(all, w) {
			return domain.TopicLiability, TopicByWording
		}
	}
	return domain.TopicSafeDriving, TopicByWording
}

// Thresholds for "quotes the Rules": nearly every word of the explanation
// must appear in one paragraph, and there must be enough words for that to
// mean something.
const (
	quoteMinWords = 8
	quoteMinShare = 0.8
)

// quotedTopic places an explanation that repeats a paragraph of the Rules
// without citing it ("Quyidagi shartlarga amal qilinganda yuk tashishga yo'l
// qo'yiladi: ..." is YHQ 162-band word for word).
func (c *TopicClassifier) quotedTopic(explanation string) string {
	words := map[string]bool{}
	for _, t := range tokenizeUz(explanation) {
		words[t] = true
	}
	if len(words) < quoteMinWords {
		return ""
	}
	best, bestShare := -1, 0.0
	for i, r := range c.rules {
		hit := 0
		for w := range words {
			if r.words[w] {
				hit++
			}
		}
		if share := float64(hit) / float64(len(words)); share > bestShare {
			best, bestShare = i, share
		}
	}
	if best < 0 || bestShare < quoteMinShare {
		return ""
	}
	p := c.rules[best].paragraph
	if p.Appendix != "" {
		n, _ := strconv.Atoi(strings.SplitN(p.Appendix, "-", 2)[0])
		if s := strings.SplitN(p.Section, ".", 2)[0]; s != "" && (n == 1 || n == 2) {
			return sectionTopic(n, s)
		}
		return c.appendixTopic(strconv.Itoa(n), "")
	}
	if m := chapterNumberRe.FindStringSubmatch(p.Chapter); m != nil {
		return c.chapterTopic(m[1], p.ClauseNumber, normalizeRef(p.Text))
	}
	return ""
}

func firstFrom(re *regexp.Regexp, text string, from int) (int, []string) {
	m := re.FindStringSubmatchIndex(text[from:])
	if m == nil {
		return -1, nil
	}
	out := make([]string, len(m)/2)
	for i := range out {
		if m[2*i] >= 0 {
			out[i] = text[from+m[2*i] : from+m[2*i+1]]
		}
	}
	return from + m[0], out
}

// appendixTopic places "N-ilova ..." using the section it names, or failing
// that the first sign or marking number after it.
func (c *TopicClassifier) appendixTopic(appendix, rest string) string {
	n, _ := strconv.Atoi(appendix)
	switch n {
	case 1, 2:
		// The section must belong to this reference, not to a later one.
		if pos, m := firstFrom(refSectionRe, rest, 0); m != nil && pos < 40 {
			if t := sectionTopic(n, m[1]); t != "" {
				return t
			}
		}
		if _, m := firstFrom(refSignRe, rest, 0); m != nil {
			return sectionTopic(n, m[1])
		}
	case 3, 4:
		for _, t := range domain.Topics {
			if t.Appendix == n && t.Section == 0 {
				return t.Key
			}
		}
	}
	return ""
}

func sectionTopic(appendix int, section string) string {
	n, _ := strconv.Atoi(section)
	for _, t := range domain.Topics {
		if t.Appendix == appendix && t.Section == n {
			return t.Key
		}
	}
	return ""
}

// chapterTopic places a chapter (and band, for the chapters the syllabus
// splits). "17¹" — the dedicated route-vehicle lane — goes with the
// route-vehicle priorities of chapter 22.
func (c *TopicClassifier) chapterTopic(chapter, band, text string) string {
	if chapter == "17¹" {
		return "route_vehicles"
	}
	n, err := strconv.Atoi(chapter)
	if err != nil {
		return ""
	}
	b, _ := strconv.Atoi(band)

	var candidates []domain.Topic
	for _, t := range domain.Topics {
		if t.Chapter == n {
			candidates = append(candidates, t)
		}
	}
	if len(candidates) == 0 {
		return ""
	}
	if len(candidates) == 1 {
		return candidates[0].Key
	}
	for _, t := range candidates {
		for _, tb := range t.Bands {
			if tb == b {
				return t.Key
			}
		}
	}
	// A split chapter cited without a band that settles it: decide by what
	// the explanation talks about.
	switch n {
	case 7:
		if strings.Contains(text, "tartibga soluvchi") {
			return "signals_regulator"
		}
		return "signals_traffic_lights"
	case 16:
		if strings.Contains(text, "yo'nalishi") && strings.Contains(text, "o'zgar") {
			return "intersections_main_road_turns"
		}
		if strings.Contains(text, "teng ahamiyatga ega bo'lmagan") || strings.Contains(text, "asosiy yo'l") {
			return "intersections_main_road"
		}
		return "intersections_equal"
	}
	return candidates[0].Key
}
