package usecase_test

import (
	"testing"

	"github.com/avtofast/avtofast-back/internal/domain"
	"github.com/avtofast/avtofast-back/internal/usecase"
	"github.com/stretchr/testify/assert"
)

func rulesForClassifier() []domain.LegalParagraph {
	p := func(chapter, clause, text string) domain.LegalParagraph {
		return domain.LegalParagraph{Document: domain.DocumentRules, Chapter: chapter, ClauseNumber: clause, Text: text}
	}
	return []domain.LegalParagraph{
		p("24-bob. Mexanik transport vositalarini shatakka olish", "144", "144. Egiluvchan ulagich bilan shatakka olishda masofa 4 — 6 metr."),
		p("17¹-bob. Alohida tasma", "115¹", "115¹. Yoʻnalishli transport vositalari uchun alohida ajratilgan tasma."),
		p("27-bob. Yuk tashish", "162", "162. Quyidagi shartlarga amal qilinganda yuk tashishga yoʻl qoʻyiladi:"),
		p("27-bob. Yuk tashish", "162", "shovqin solmasa, chang koʻtarmasa, yoʻlni va atrof-muhitni ifloslantirmasa;"),
		p("27-bob. Yuk tashish", "162", "transport vositasining turgʻunligini buzmasa va uni boshqarishni qiyinlashtirmasa."),
	}
}

func TestTopicClassifier_FilesByTheCitedClause(t *testing.T) {
	c := usecase.NewTopicClassifier(rulesForClassifier())
	cases := []struct {
		explanation, topic, how string
	}{
		{"YHQ 16-bobi 105-bandiga asosan, teng ahamiyatga ega boʻlgan yoʻllar ...", "intersections_equal", usecase.TopicByChapter},
		{"YHQ 16-bobi 104 bandiga asosan, teng ahamiyatga ega boʻlmagan yoʻllar ...", "intersections_main_road", usecase.TopicByChapter},
		{"YHQ 16-bobi 106-,105 - va 104 - bandlariga asosan, asosiy yoʻlning yoʻnalishi ...", "intersections_main_road_turns", usecase.TopicByChapter},
		{"YHQ 7-bobi 38-bandiga asosan, tartibga soluvchining oʻng qoʻli ...", "signals_regulator", usecase.TopicByChapter},
		{"YHQ 7-bobi 32-bandiga asosan, yoʻnaltirgich koʻrinishidagi ...", "signals_traffic_lights", usecase.TopicByChapter},
		{"YHQ 26 – bobi 153 – bandiga asosan: Odam tashish ...", "passenger_transport", usecase.TopicByChapter},
		{"YHQ 4-bobi 17-bandiga asosan, piyodalarga ...", "pedestrians", usecase.TopicByChapter},
		{"YHQ 15-bobi 100 - va 101 - bandlariga asosan, svetoforning ...", "intersections_regulated", usecase.TopicByChapter},
		// A band alone, placed by the Rules' text.
		{"Yoʻl harakati qoidalarining 144 bandiga asosan, egiluvchan ulagich ...", "towing", usecase.TopicByBand},
		// Appendices: the section named, or else the sign number.
		{"YHQ 1-ilovasining 3-boʻlimi 3.16 belgisiga asosan, “Eng kam oraliq” ...", "signs_prohibitory", usecase.TopicByAppendix},
		{"YHQ 1-ilovasidagi 7.4.1 “Transport vositasining turi” qoʻshimcha axborot belgisi, 3.28 ...", "signs_additional", usecase.TopicByAppendix},
		{"YHQ 1-ilovasi 3-boʻlimi 3-xatboshiga asosan, Qoidalarning 176-bandiga binoan ... 7.18 ... 3.27", "signs_prohibitory", usecase.TopicByAppendix},
		{"YHQ 2-ilovasining 1-boʻlimiga asosan, 1.21 yotiq chizigʻi ...", "markings_horizontal", usecase.TopicByAppendix},
		{"YHQ 3 – ilovasi 7.10 – bandiga asosan: Transport vositalari ...", "vehicle_faults", usecase.TopicByAppendix},
		// The earliest reference wins over later cross-references.
		{"YHQ 29-bobi 176-bandi 9-izohiga asosan, “Uzun oʻlchamli transport vositasi” — 2.1 belgisi ...", "registration_marks", usecase.TopicByChapter},
	}
	for _, tc := range cases {
		topic, how := c.Classify("", tc.explanation)
		assert.Equal(t, tc.topic, topic, tc.explanation)
		assert.Equal(t, tc.how, how, tc.explanation)
	}
}

func TestTopicClassifier_WithoutACitation(t *testing.T) {
	c := usecase.NewTopicClassifier(rulesForClassifier())

	// Quotes YHQ 162 across its list paragraphs without citing it.
	topic, how := c.Classify("", "Quyidagi shartlarga amal qilinganda yuk tashishga yoʻl qoʻyiladi: shovqin solmasa, chang koʻtarmasa, yoʻlni va atrof-muhitni ifloslantirmasa; transport vositasining turgʻunligini buzmasa.")
	assert.Equal(t, "cargo_transport", topic)
	assert.Equal(t, usecase.TopicByQuote, how)

	topic, how = c.Classify("", "Suyaklar singanda shikastlangan qoʻl yoki oyoqni har qanday qattiq vosita yordamida mustahkamlab, harakatsizlantirish kerak.")
	assert.Equal(t, domain.TopicFirstAid, topic)
	assert.Equal(t, usecase.TopicByWording, how)

	topic, _ = c.Classify("", "Inersiya kuchi taʼsiri ostida tormozlash vaqtida oʻqlarga tushadigan ogʻirlik qayta taqsimlanadi.")
	assert.Equal(t, domain.TopicSafeDriving, topic)

	// A sign number with no appendix named still names its section.
	topic, how = c.Classify("", "4.3 “Aylanma harakatlanish” belgisi koʻrsatkichlar yoʻnalishida harakatlanishga ruxsat beradi.")
	assert.Equal(t, "signs_mandatory", topic)
	assert.Equal(t, usecase.TopicBySign, how)
}

func TestTopicClassifier_SplitChapterWithoutBand(t *testing.T) {
	c := usecase.NewTopicClassifier(nil)
	topic, _ := c.Classify("", "YHQ 7-bobiga koʻra tartibga soluvchining ishoralari svetofor ishoralaridan ustun.")
	assert.Equal(t, "signals_regulator", topic)
	topic, _ = c.Classify("", "YHQ 16-bobiga koʻra asosiy yoʻlning yoʻnalishi chorrahada oʻzgarganda ...")
	assert.Equal(t, "intersections_main_road_turns", topic)
}

func TestTopics_EveryKeyIsUniqueAndGrouped(t *testing.T) {
	seen := map[string]bool{}
	for _, topic := range domain.Topics {
		assert.False(t, seen[topic.Key], "duplicate topic %s", topic.Key)
		seen[topic.Key] = true
		assert.NotEmpty(t, domain.TopicGroupTitle(topic.Group), "topic %s has no group", topic.Key)
		assert.True(t, domain.IsKnownCategory(topic.Key))
	}
	assert.False(t, domain.IsKnownCategory(domain.CategoryRoadSigns), "the old coarse categories are not topics")
}
