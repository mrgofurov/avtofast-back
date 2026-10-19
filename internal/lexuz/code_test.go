package lexuz

import (
	"os"
	"strings"
	"testing"

	"github.com/avtofast/avtofast-back/internal/domain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func codeFixture() []byte {
	var b strings.Builder
	b.WriteString(elem("ACT_TITLE", "-1", "Maʼmuriy javobgarlik toʻgʻrisidagi kodeks"))
	b.WriteString(elem("TEXT_HEADER_DEFAULT", "-2", "MAXSUS QISM"))
	b.WriteString(elem("TEXT_HEADER_DEFAULT", "-3", "X bob. QISHLOQ XOʻJALIGIDAGI HUQUQBUZARLIK"))
	b.WriteString(elem("CLAUSE_DEFAULT", "-4", "100-modda. Not traffic"))
	b.WriteString(elem("ACT_TEXT", "-5", "Some farm rule."))
	b.WriteString(elem("TEXT_HEADER_DEFAULT", "-6", "XI bob. TRANSPORTDAGI, YOʻL XOʻJALIGI VA ALOQA SOHALARIDAGI HUQUQBUZARLIKLAR"))
	b.WriteString(elem("CLAUSE_DEFAULT", "-60", "116-modda. Uchish xavfsizligi qoidalarini buzish"))
	b.WriteString(elem("ACT_TEXT", "-61", "Aviation, not roads."))
	b.WriteString(elem("CLAUSE_DEFAULT", "-7", "128<sup>3</sup>-modda. Belgilangan harakat tezligini oshirib yuborish"))
	b.WriteString(elem("BY_DEFAULT", "-8", "(amendment note)"))
	b.WriteString(elem("ACT_TEXT", "-9", ""))
	b.WriteString(elem("ACT_TEXT", "-10", "Tezlikni soatiga 20 kilometrdan koʻp boʻlmagan kattalikda oshirib yuborish —"))
	b.WriteString(elem("ACT_TEXT", "-11", "bazaviy hisoblash miqdorining bir baravari miqdorida jarima solishga sabab boʻladi."))
	b.WriteString(elem("CLAUSE_DEFAULT", "-12", "128<sup>4</sup>-modda.Svetoforning taqiqlovchi signaliga oʻtish"))
	b.WriteString(elem("ACT_TEXT", "-13", "Taqiqlovchi signalga oʻtish jarimaga sabab boʻladi."))
	b.WriteString(elem("CLAUSE_DEFAULT", "-62", "155<sup>4</sup>-modda. Kripto-aktivlar aylanmasi sohasidagi qonunchilikni buzish"))
	b.WriteString(elem("ACT_TEXT", "-63", "Crypto, not roads."))
	b.WriteString(elem("TEXT_HEADER_DEFAULT", "-14", "XII bob. TURAR JOY HUQUQLARIGA TAALLUQLI HUQUQBUZARLIKLAR"))
	b.WriteString(elem("CLAUSE_DEFAULT", "-15", "156-modda. Not traffic either"))
	b.WriteString(elem("ACT_TEXT", "-16", "Housing."))
	b.WriteString(elem("SIGNATURE", "-17", "Prezident"))
	return []byte(b.String())
}

func TestParseTrafficOffences_KeepsRoadArticlesOfChapterXIOnly(t *testing.T) {
	ps, err := parseTrafficOffences(codeFixture())
	require.NoError(t, err)

	type row struct{ id, number, section, text string }
	var got []row
	for _, p := range ps {
		got = append(got, row{p.SourceElementID, p.ClauseNumber, p.Section, p.Text})
		assert.True(t, strings.HasPrefix(p.Chapter, "XI bob"), p.Chapter)
	}
	h1 := "128³-modda. Belgilangan harakat tezligini oshirib yuborish"
	h2 := "128⁴-modda.Svetoforning taqiqlovchi signaliga oʻtish"
	assert.Equal(t, []row{
		{"-7", "128³", h1, h1},
		{"-10", "128³", h1, "Tezlikni soatiga 20 kilometrdan koʻp boʻlmagan kattalikda oshirib yuborish —"},
		{"-11", "128³", h1, "bazaviy hisoblash miqdorining bir baravari miqdorida jarima solishga sabab boʻladi."},
		{"-12", "128⁴", h2, h2},
		{"-13", "128⁴", h2, "Taqiqlovchi signalga oʻtish jarimaga sabab boʻladi."},
	}, got)
}

func TestDocumentParse_StampsDocumentFields(t *testing.T) {
	law, _ := DocumentByCode(domain.DocumentLaw)
	law.minParagraphs = 1
	ps, err := law.Parse(codeFixture())
	require.NoError(t, err)
	require.NotEmpty(t, ps)
	for i, p := range ps {
		assert.Equal(t, domain.DocumentLaw, p.Document)
		assert.Equal(t, "Yoʻl harakati toʻgʻrisidagi qonun", p.DocumentTitle)
		assert.Equal(t, "https://lex.uz/docs/-6764454", p.SourceURL)
		assert.Equal(t, i+1, p.Ordinal)
	}
	// The law keeps every chapter.
	assert.Equal(t, "-4", ps[0].SourceElementID)
}

func TestParse_SavedLawAndCodePages(t *testing.T) {
	for code, file := range map[string]string{
		domain.DocumentLaw:  "../../data/lex/lex-6764454.html",
		domain.DocumentCode: "../../data/lex/lex-97664.html",
	} {
		page, err := os.ReadFile(file)
		if err != nil {
			t.Skip("no saved lex.uz page")
		}
		doc, _ := DocumentByCode(code)
		ps, err := doc.Parse(page)
		require.NoError(t, err, code)

		articles := map[string]string{}
		for _, p := range ps {
			if _, ok := articles[p.ClauseNumber]; !ok {
				articles[p.ClauseNumber] = p.Section
			}
		}
		if code == domain.DocumentLaw {
			assert.Contains(t, articles["4"], "Asosiy tushunchalar")
		} else {
			assert.Contains(t, articles["128³"], "tezligini oshirib")
			assert.Contains(t, articles["131"], "mastlik")
			assert.NotContains(t, articles, "156", "chapter XII is not traffic")
			assert.NotContains(t, articles, "116", "aviation is not road traffic")
			assert.NotContains(t, articles, "155⁴", "crypto-assets are not road traffic")
		}
	}
}
