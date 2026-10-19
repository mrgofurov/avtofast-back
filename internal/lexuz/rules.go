package lexuz

import (
	"regexp"
	"strconv"
	"strings"

	"github.com/avtofast/avtofast-back/internal/domain"
)

var (
	// The decree's own appendices ("qaroriga 1-ILOVA" is the Rules) and the
	// Rules' appendices ("qoidalariga 1-ILOVA" is road signs).
	decreeAppendixRe = regexp.MustCompile(`qaroriga\s+(\d+)-ILOVA`)
	rulesAppendixRe  = regexp.MustCompile(`qoidalariga\s+(\d+)-ILOVA`)

	chapterRe = regexp.MustCompile(`^\d+\S*-bob\.`)
	// "105. Teng ahamiyatga..." in the main text; "115¹." for clauses added
	// by later amendments.
	mainClauseRe = regexp.MustCompile(`^(\d+)([` + superscriptDigits + `]*)\.\s`)
	// "2.4. “Yo'l bering”", "1.14.1 — (oq rangli “zebra”)", "2.3.2, 2.3.3. ...",
	// "1.2 (enli sidirgʻa chiziq) — ...", "1.29.1-sunʼiy notekislik" in the
	// appendices. What follows the number tells an entry from a remark that
	// merely opens with sign numbers ("1.1, 1.2, 1.5 — 1.32 belgilar ...").
	appendixNumberRe = regexp.MustCompile(`^(\d+(?:\.\d+)+(?:\s*[,–—-]\s*\d+(?:\.\d+)+)*)`)
	appendixEntryRe  = regexp.MustCompile(`^(?:\.\s|\.$|$|\s*[—–]\s|\s+\(|-\pL)`)
	// "1. Tormoz tizimlari": a section heading written as a paragraph.
	appendixSectionRe = regexp.MustCompile(`^\d+\.\s+\S`)
)

// parseRules reads the Yo'l harakati qoidalari: the main text's chapters
// and the Rules' appendices (signs, markings, vehicle faults, danger plates).
// The decree around them (its preamble, signature and list of repealed acts)
// is left out, as are tables and images that lex.uz renders without text.
func parseRules(page []byte) ([]domain.LegalParagraph, error) {
	var (
		out      []domain.LegalParagraph
		inRules  bool
		chapter  string
		appendix string
		section  string
		clause   string
		lastMain int
	)

	for _, e := range elements(page) {
		class, id, text := e.class, e.id, e.text

		if class == "APPL_BANNER_LANDSCAPE_TITLE" {
			if n := rulesAppendixRe.FindStringSubmatch(text); n != nil && inRules {
				appendix, chapter, section, clause = n[1]+"-ilova", "", "", ""
				continue
			}
			if n := decreeAppendixRe.FindStringSubmatch(text); n != nil {
				if n[1] != "1" && inRules {
					break // the decree's next appendix: the Rules are over
				}
				inRules = n[1] == "1"
			}
			continue
		}
		if !inRules || text == "" {
			continue
		}

		switch class {
		case "ACT_TITLE_APPL":
			if appendix != "" && !strings.Contains(appendix, ".") {
				appendix += ". " + text
			}
			continue
		case "TEXT_HEADER_DEFAULT":
			if appendix == "" && chapterRe.MatchString(text) {
				chapter, clause = text, ""
			} else if appendix != "" {
				section, clause = text, ""
			}
			continue
		case "ACT_TEXT", "TEXT_BOLD_CENTER", "TEXT_CENTER", "FOOTNOTE":
		default:
			// BY_DEFAULT holds tables and amendment notes; the rest is the
			// decree's own furniture.
			continue
		}

		if appendix == "" {
			if chapter == "" {
				continue
			}
			// Clause numbers only go up; a "3." inside a clause's list is
			// part of that clause, not a new one.
			if n := mainClauseRe.FindStringSubmatch(text); n != nil {
				v, _ := strconv.Atoi(n[1])
				if v > lastMain || (n[2] != "" && v == lastMain) {
					clause, lastMain = n[1]+n[2], v
				}
			}
		} else {
			if n := appendixNumberRe.FindStringSubmatch(text); n != nil {
				if appendixEntryRe.MatchString(text[len(n[0]):]) {
					clause = strings.TrimSpace(n[1])
				} else {
					// A remark about several signs belongs to the section,
					// not to whichever sign came before it.
					clause = ""
				}
			} else if class == "ACT_TEXT" && appendixSectionRe.MatchString(text) && len([]rune(text)) <= 80 {
				section, clause = text, ""
				continue
			}
		}

		out = append(out, domain.LegalParagraph{
			SourceElementID: id,
			Chapter:         chapter,
			Appendix:        appendix,
			Section:         section,
			ClauseNumber:    clause,
			Text:            text,
		})
	}

	return out, nil
}
