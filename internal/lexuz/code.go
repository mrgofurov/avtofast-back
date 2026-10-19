package lexuz

import (
	"regexp"
	"strconv"
	"strings"

	"github.com/avtofast/avtofast-back/internal/domain"
)

var (
	// "1-bob. Umumiy qoidalar" in a law, "XI bob. TRANSPORTDAGI ..." in the
	// code. Parts ("BIRINCHI BOʻLIM"), "MAXSUS QISM" and "1-§." headings sit
	// above or below chapters and do not change which chapter an article is in.
	codeChapterRe = regexp.MustCompile(`^[0-9IVXLC]+[` + superscriptDigits + `]*\s*-?\s*(?i:bob)\b`)
	// "128³-modda. Transport vositalari ..." (sometimes with no space after
	// the dot).
	articleRe = regexp.MustCompile(`^(\d+[` + superscriptDigits + `]*)-modda\b`)
)

// parseArticles reads a law or code: chapters, articles ("N-modda") and the
// paragraphs of each article. The article heading is kept as the article's
// first paragraph, since it names what the article is about. Reading stops
// at the signature or the first appendix banner.
func parseArticles(page []byte, keep func(chapter, article string) bool) []domain.LegalParagraph {
	var (
		out     []domain.LegalParagraph
		chapter string
		heading string
		number  string
	)
	for _, e := range elements(page) {
		if e.class == "SIGNATURE" || e.class == "APPL_BANNER_LANDSCAPE_TITLE" {
			break
		}
		if e.text == "" {
			continue
		}
		switch e.class {
		case "TEXT_HEADER_DEFAULT":
			if codeChapterRe.MatchString(e.text) {
				chapter, heading, number = e.text, "", ""
			}
			continue
		case "CLAUSE_DEFAULT":
			n := articleRe.FindStringSubmatch(e.text)
			if n == nil {
				continue
			}
			heading, number = e.text, n[1]
		case "ACT_TEXT":
			if number == "" {
				continue // preamble, or text between a chapter and its first article
			}
		default:
			continue // BY_DEFAULT holds tables and amendment notes
		}
		if chapter == "" || !keep(chapter, number) {
			continue
		}
		out = append(out, domain.LegalParagraph{
			SourceElementID: e.id,
			Chapter:         chapter,
			Section:         heading,
			ClauseNumber:    number,
			Text:            e.text,
		})
	}
	return out
}

func parseLaw(page []byte) ([]domain.LegalParagraph, error) {
	return parseArticles(page, func(string, string) bool { return true }), nil
}

// Road transport and roads within chapter XI: articles 121 (damage inside
// vehicles) to 149 (road protection zones). Before them come rail, air and
// water transport; after them pipelines, telecoms and crypto-assets.
const firstRoadArticle, lastRoadArticle = 121, 149

// parseTrafficOffences keeps the road-traffic offences of the Ma'muriy
// javobgarlik to'g'risidagi kodeks, which is where every traffic fine is set.
// The rest of the code (tax, trade, elections, aviation...) has nothing a
// learner driver needs, and its words ("aylanma" in crypto-asset turnover)
// would only crowd out the articles that matter.
func parseTrafficOffences(page []byte) ([]domain.LegalParagraph, error) {
	return parseArticles(page, func(chapter, article string) bool {
		if !strings.HasPrefix(chapter, "XI bob") {
			return false
		}
		n, err := strconv.Atoi(strings.TrimRight(article, superscriptDigits))
		return err == nil && n >= firstRoadArticle && n <= lastRoadArticle
	}), nil
}
