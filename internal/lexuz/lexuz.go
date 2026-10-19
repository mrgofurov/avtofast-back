// Package lexuz reads official documents as lex.uz publishes them into
// paragraphs that know where they sit: chapter, appendix, article or clause.
// These are the texts the AI tutor quotes, so only the documents' own words
// are kept.
package lexuz

import (
	"fmt"
	"html"
	"regexp"
	"strings"

	"github.com/avtofast/avtofast-back/internal/domain"
)

// Document is one official text the importer knows how to read.
type Document struct {
	Code  string // domain.DocumentRules, DocumentLaw, DocumentCode
	Title string // how citations name it
	URL   string
	parse func(page []byte) ([]domain.LegalParagraph, error)
	// A page yielding fewer paragraphs is refused: the importer replaces the
	// stored document, and a lex.uz layout change must not quietly empty it.
	minParagraphs int
}

var Documents = []Document{
	{Code: domain.DocumentRules, Title: "YHQ", URL: "https://lex.uz/docs/-5953883", parse: parseRules, minParagraphs: 800},
	{Code: domain.DocumentLaw, Title: "Yoʻl harakati toʻgʻrisidagi qonun", URL: "https://lex.uz/docs/-6764454", parse: parseLaw, minParagraphs: 200},
	{Code: domain.DocumentCode, Title: "MJtK", URL: "https://lex.uz/docs/-97664", parse: parseTrafficOffences, minParagraphs: 200},
}

func DocumentByCode(code string) (Document, bool) {
	for _, d := range Documents {
		if d.Code == code {
			return d, true
		}
	}
	return Document{}, false
}

// Parse reads a saved or fetched lex.uz page of this document.
func (d Document) Parse(page []byte) ([]domain.LegalParagraph, error) {
	out, err := d.parse(page)
	if err != nil {
		return nil, err
	}
	if len(out) < d.minParagraphs {
		return nil, fmt.Errorf("%s: page yields %d paragraphs, expected at least %d; is it the right page, or has lex.uz changed its layout?", d.Code, len(out), d.minParagraphs)
	}
	for i := range out {
		out[i].Document, out[i].DocumentTitle, out[i].SourceURL = d.Code, d.Title, d.URL
		out[i].Ordinal = i + 1
	}
	return out, nil
}

// Every lex.uz paragraph is `<div class="CLASS lx_elem" ...>` holding a
// toolbar and then `<div name="-ID" id="-ID">TEXT</div>`. Editor notes
// ("Oldingi tahrirga qarang"), classifier tags and the toolbar live outside
// that inner div, so taking only its content leaves the document's own words.
var elementRe = regexp.MustCompile(`(?s)<div class="([A-Z_]+) lx_elem"[^>]*>.*?<div name="(-\d+)" id="-\d+">(.*?)</div></div>`)

var (
	tagRe   = regexp.MustCompile(`<[^>]+>`)
	supRe   = regexp.MustCompile(`(?s)<sup>(.*?)</sup>`)
	spaceRe = regexp.MustCompile(`\s+`)
)

var superscript = strings.NewReplacer("0", "⁰", "1", "¹", "2", "²", "3", "³", "4", "⁴", "5", "⁵", "6", "⁶", "7", "⁷", "8", "⁸", "9", "⁹")

const superscriptDigits = "⁰¹²³⁴⁵⁶⁷⁸⁹"

type element struct {
	class, id, text string
}

func elements(page []byte) []element {
	matches := elementRe.FindAllSubmatch(page, -1)
	out := make([]element, len(matches))
	for i, m := range matches {
		out[i] = element{class: string(m[1]), id: string(m[2]), text: cleanText(string(m[3]))}
	}
	return out
}

func cleanText(inner string) string {
	// "17<sup>1</sup>-bob" is chapter 17¹ and "128<sup>3</sup>-modda" article
	// 128³, not 171 and 1283.
	inner = supRe.ReplaceAllStringFunc(inner, func(m string) string {
		return superscript.Replace(supRe.FindStringSubmatch(m)[1])
	})
	text := html.UnescapeString(tagRe.ReplaceAllString(inner, " "))
	return strings.TrimSpace(spaceRe.ReplaceAllString(text, " "))
}
