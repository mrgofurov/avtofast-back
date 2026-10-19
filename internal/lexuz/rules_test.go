package lexuz

import (
	"fmt"
	"os"
	"strings"
	"testing"

	"github.com/avtofast/avtofast-back/internal/domain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// elem renders one paragraph the way lex.uz does, toolbar included.
func elem(class, id, inner string) string {
	return fmt.Sprintf(`<div class="%s lx_elem" onmousemove="lx_mo(event,%s)"><div class="lx_elem2"><div class="lx_elem3"><span onclick="lx_sf(%s)">Ҳужжатга таклиф юбориш</span></div></div><div name="%s" id="%s">%s</div></div>`,
		class, id, id, id, id, inner)
}

func fixture() []byte {
	var b strings.Builder
	b.WriteString(elem("ACT_TEXT", "-1", "1. Decree preamble, not a rule."))
	b.WriteString(elem("APPL_BANNER_LANDSCAPE_TITLE", "-2", "Vazirlar Mahkamasining 2022-yil 12-apreldagi 172-son qaroriga 1-ILOVA"))
	b.WriteString(elem("ACT_TITLE_APPL", "-3", "Yoʻl harakati qoidalari"))
	b.WriteString(elem("TEXT_HEADER_DEFAULT", "-4", "16-bob. Tartibga solinmagan chorrahalar"))
	b.WriteString(`<div class="COMMENT lx_no_select"><div><a href="#">Oldingi</a> tahrirga qarang.</div></div>`)
	b.WriteString(elem("ACT_TEXT", "-5", "105. Teng ahamiyatga ega boʻlgan yoʻllar kesishgan chorrahada &mdash; oʻngdan."))
	b.WriteString(elem("ACT_TEXT", "-6", "Bunday chorrahalarda <a href=\"#\">tramvay</a> oldin oʻtadi."))
	b.WriteString(elem("ACT_TEXT", "-7", "3. A numbered list item inside clause 105."))
	b.WriteString(elem("BY_DEFAULT", "-8", "<table><tr><td>table</td></tr></table>"))
	b.WriteString(elem("TEXT_HEADER_DEFAULT", "-9", "17<sup>1</sup>-bob. Alohida tasma"))
	b.WriteString(elem("ACT_TEXT", "-10", "115<sup>1</sup>. Amended clause."))
	b.WriteString(elem("APPL_BANNER_LANDSCAPE_TITLE", "-11", "Yoʻl harakati qoidalariga 1-ILOVA"))
	b.WriteString(elem("ACT_TITLE_APPL", "-12", "Yoʻl belgilari"))
	b.WriteString(elem("TEXT_HEADER_DEFAULT", "-13", "2. Imtiyoz belgilari"))
	b.WriteString(elem("ACT_TEXT", "-14", "Imtiyoz belgilari harakatlanish navbatini belgilaydi."))
	b.WriteString(elem("ACT_TEXT", "-15", "2.4. “Yoʻl bering!”. Haydovchi yoʻl berishi kerak."))
	b.WriteString(elem("ACT_TEXT", "-16", "Belgi chorraha oldiga oʻrnatiladi."))
	b.WriteString(elem("ACT_TEXT", "-17", "2.1, 2.4 belgilari aholi punktlarida ham qoʻllanadi."))
	b.WriteString(elem("APPL_BANNER_LANDSCAPE_TITLE", "-18", "Yoʻl harakati qoidalariga 2-ILOVA"))
	b.WriteString(elem("TEXT_HEADER_DEFAULT", "-19", "1. Yotiq chiziqlar"))
	b.WriteString(elem("ACT_TEXT", "-20", "1.2 (enli sidirgʻa chiziq) — qatnov qismi chetini bildiradi."))
	b.WriteString(elem("ACT_TEXT", "-21", "1.14.1 — (oq rangli “zebra”) — piyodalarning oʻtish joyi."))
	b.WriteString(elem("APPL_BANNER_LANDSCAPE_TITLE", "-22", "Vazirlar Mahkamasining 2022-yil 12-apreldagi 172-son qaroriga 2-ILOVA"))
	b.WriteString(elem("ACT_TEXT", "-23", "1. A repealed decree."))
	return []byte(b.String())
}

func TestParse_Structure(t *testing.T) {
	ps, err := parseRules(fixture())
	require.NoError(t, err)

	type row struct{ id, chapter, appendix, section, clause string }
	var got []row
	for _, p := range ps {
		got = append(got, row{p.SourceElementID, p.Chapter, p.Appendix, p.Section, p.ClauseNumber})
	}
	ch16, ch17 := "16-bob. Tartibga solinmagan chorrahalar", "17¹-bob. Alohida tasma"
	signs, marks := "1-ilova. Yoʻl belgilari", "2-ilova"
	assert.Equal(t, []row{
		{"-5", ch16, "", "", "105"},
		{"-6", ch16, "", "", "105"},
		{"-7", ch16, "", "", "105"}, // "3." does not go back in numbering
		{"-10", ch17, "", "", "115¹"},
		{"-14", "", signs, "2. Imtiyoz belgilari", ""},
		{"-15", "", signs, "2. Imtiyoz belgilari", "2.4"},
		{"-16", "", signs, "2. Imtiyoz belgilari", "2.4"},
		{"-17", "", signs, "2. Imtiyoz belgilari", ""}, // a remark, not sign 2.1
		{"-20", "", marks, "1. Yotiq chiziqlar", "1.2"},
		{"-21", "", marks, "1. Yotiq chiziqlar", "1.14.1"},
	}, got)

	assert.Equal(t, "105. Teng ahamiyatga ega boʻlgan yoʻllar kesishgan chorrahada — oʻngdan.", ps[0].Text)
	assert.Equal(t, "Bunday chorrahalarda tramvay oldin oʻtadi.", ps[1].Text)
	for _, p := range ps {
		assert.NotContains(t, p.Text, "Ҳужжатга")
		assert.NotContains(t, p.Text, "Oldingi")
	}
}

func TestParse_RefusesAPageThatIsNotTheRules(t *testing.T) {
	rules, _ := DocumentByCode(domain.DocumentRules)
	_, err := rules.Parse(fixture()) // far too short to be the real document
	assert.Error(t, err)
}

// Saved copies are not committed (data/ is gitignored); run
// `go run ./cmd/lex-import -save data/lex -dry-run` to get them.
func TestParse_SavedRulesPage(t *testing.T) {
	page, err := os.ReadFile("../../data/lex/lex-5953883.html")
	if err != nil {
		t.Skip("no saved lex.uz page")
	}
	rules, _ := DocumentByCode(domain.DocumentRules)
	ps, err := rules.Parse(page)
	require.NoError(t, err)

	byClause := map[string]string{}
	for _, p := range ps {
		key := p.Chapter + p.Appendix + "|" + p.ClauseNumber
		if _, ok := byClause[key]; !ok {
			byClause[key] = p.Text
		}
	}
	assert.Contains(t, byClause["16-bob. Tartibga solinmagan chorrahalar|105"], "oʻngdan yaqinlashayotgan")
	assert.Contains(t, byClause["1-ilova. Yoʻl belgilari|2.4"], "Yoʻl bering")
}
