package main

import (
	"context"
	"fmt"
	"time"

	"github.com/avtofast/avtofast-back/internal/config"
	"github.com/avtofast/avtofast-back/internal/domain"
	"github.com/avtofast/avtofast-back/internal/repository/postgres"
	"github.com/avtofast/avtofast-back/pkg/crypto"
	"github.com/avtofast/avtofast-back/pkg/logger"
)

func main() {
	log := logger.DefaultLogger
	log.Info("Starting database seed process...")

	cfg, err := config.Load("config/config.yaml")
	if err != nil {
		log.Error("Failed to load config", err)
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	db, err := postgres.NewPool(ctx, cfg)
	if err != nil {
		log.Error("Failed to connect to postgres for seeding", err)
		return
	}
	defer db.Close()

	contentRepo := postgres.NewContentRepository(db)

	now := time.Now().UTC()
	manifestPayload := "uz-theory-2026-09:2026.09.1:" + now.Format(time.RFC3339)
	sha := crypto.SHA256Hex([]byte(manifestPayload))
	sig := crypto.SignEd25519(cfg.Ed25519PrivKey, []byte(sha))

	pack := &domain.QuestionPack{
		PackID:            "uz-theory-2026-09",
		Version:           "2026.09.1",
		Title:             "O'zbekiston haydovchilik nazariyasi",
		Locales:           []string{domain.LocaleUzLatn, domain.LocaleUzCyrl, domain.LocaleRu, domain.LocaleEn},
		QuestionCount:     7,
		DownloadBytes:     18432000,
		MandatoryUpdate:   false,
		Status:            "published",
		ManifestSHA256:    sha,
		ManifestSignature: sig,
		PublishedAt:       &now,
	}

	existingPack, _ := contentRepo.GetPackByID(ctx, pack.PackID)
	if existingPack == nil {
		if err := contentRepo.CreatePack(ctx, pack); err != nil {
			log.Error("Failed to create pack", err)
			return
		}
		log.Info("Created published question pack: " + pack.PackID)
	} else {
		pack.ID = existingPack.ID
	}

	// Sample seed questions covering different categories & 4 locales
	sampleQuestions := []*domain.Question{
		{
			PublicID:       "sign-014",
			PackTableID:    pack.ID,
			PackID:         pack.PackID,
			ContentVersion: pack.Version,
			Category:       domain.CategoryRoadSigns,
			Difficulty:     domain.DifficultyEasy,
			Image: &domain.QuestionImage{
				URL:    "https://api.avtofast.uz/assets/questions/sign-014.webp",
				SHA256: "d3b07384d113edec49eaa6238ad5ff00",
				Alt:    map[string]string{domain.LocaleUzLatn: "Sariq romb shaklidagi yo'l belgisi"},
			},
			Source: &domain.QuestionSource{
				Reference:         "YHQ, 2.1 — Asosiy yo'l",
				EffectiveFrom:     "2026-09-01",
				OfficialSourceURL: "https://lex.uz/docs/5960010",
			},
			CorrectChoiceID: "a",
			Status:          "published",
			Translations: map[string]domain.QuestionTranslationData{
				domain.LocaleUzLatn: {
					Prompt: "Ushbu yo'l belgisi nimani bildiradi?",
					Choices: []domain.ChoiceItem{
						{ID: "a", Text: "Asosiy yo'l", Position: 1},
						{ID: "b", Text: "Yo'l bering", Position: 2},
						{ID: "c", Text: "To'xtamasdan harakatlanish taqiqlangan", Position: 3},
					},
					Explanation: "2.1 'Asosiy yo'l' belgisi tartibga solinmagan chorrahalardan imtiyozli o'tish huquqini beradi.",
				},
				domain.LocaleUzCyrl: {
					Prompt: "Ушбу йўл белгиси нимани билдиради?",
					Choices: []domain.ChoiceItem{
						{ID: "a", Text: "Асосий йўл", Position: 1},
						{ID: "b", Text: "Йўл беринг", Position: 2},
						{ID: "c", Text: "Тўхтамасдан ҳаракатланиш тақиқланган", Position: 3},
					},
					Explanation: "2.1 'Асосий йўл' белгиси тартибга солинмаган чорраҳалардан имтиёзли ўтиш ҳуқуқини беради.",
				},
				domain.LocaleRu: {
					Prompt: "Что означает данный дорожный знак?",
					Choices: []domain.ChoiceItem{
						{ID: "a", Text: "Главная дорога", Position: 1},
						{ID: "b", Text: "Уступите дорогу", Position: 2},
						{ID: "c", Text: "Движение без остановки запрещено", Position: 3},
					},
					Explanation: "Знак 2.1 'Главная дорога' предоставляет право преимущественного проезда нерегулируемых перекрестков.",
				},
				domain.LocaleEn: {
					Prompt: "What does this road sign indicate?",
					Choices: []domain.ChoiceItem{
						{ID: "a", Text: "Priority road", Position: 1},
						{ID: "b", Text: "Give way", Position: 2},
						{ID: "c", Text: "Stop sign", Position: 3},
					},
					Explanation: "Sign 2.1 'Priority road' grants priority at uncontrolled intersections.",
				},
			},
		},
		{
			PublicID:       "cross-027",
			PackTableID:    pack.ID,
			PackID:         pack.PackID,
			ContentVersion: pack.Version,
			Category:       domain.CategoryIntersections,
			Difficulty:     domain.DifficultyMedium,
			Source: &domain.QuestionSource{
				Reference:     "YHQ, 15.2 — Chorrahalardan o'tish",
				EffectiveFrom: "2026-09-01",
			},
			CorrectChoiceID: "a",
			Status:          "published",
			Translations: map[string]domain.QuestionTranslationData{
				domain.LocaleUzLatn: {
					Prompt: "Teng ahamiyatli yo'llar kesishmasida qaysi haydovchi birinchi bo'lib o'tadi?",
					Choices: []domain.ChoiceItem{
						{ID: "a", Text: "O'ng tomondan to'siq bo'lmagan haydovchi", Position: 1},
						{ID: "b", Text: "Tezlikni birinchi oshirgan haydovchi", Position: 2},
					},
					Explanation: "Teng ahamiyatli yo'llar kesishmasida haydovchi o'ngdan kelayotgan transport vositasiga yo'l berishi shart.",
				},
				domain.LocaleUzCyrl: {
					Prompt: "Тенг аҳамиятли йўллар кесишмасида қайси ҳайдовчи биринчи бўлиб ўтади?",
					Choices: []domain.ChoiceItem{
						{ID: "a", Text: "Ўнг томондан тўсиқ бўлмаган ҳайдовчи", Position: 1},
						{ID: "b", Text: "Тезликни биринчи оширган ҳайдовчи", Position: 2},
					},
					Explanation: "Тенг аҳамиятли йўллар кесишмасида ҳайдовчи ўнгдан келаётган транспорт воситасига йўл бериши шарт.",
				},
				domain.LocaleRu: {
					Prompt: "Кто имеет право преимущественного проезда на равнозначном перекрестке?",
					Choices: []domain.ChoiceItem{
						{ID: "a", Text: "Водитель, не имеющий помехи справа", Position: 1},
						{ID: "b", Text: "Водитель, первым набравший скорость", Position: 2},
					},
					Explanation: "На равнозначном перекрестке действует правило помехи справа.",
				},
				domain.LocaleEn: {
					Prompt: "Who has the right of way at an equal intersection?",
					Choices: []domain.ChoiceItem{
						{ID: "a", Text: "Driver with no vehicle approaching from the right", Position: 1},
						{ID: "b", Text: "The fastest vehicle", Position: 2},
					},
					Explanation: "At an uncontrolled intersection of equal roads, yield to traffic on your right.",
				},
			},
		},
	}

	for _, q := range sampleQuestions {
		existingQ, _ := contentRepo.GetQuestionByID(ctx, q.PublicID)
		if existingQ == nil {
			if err := contentRepo.CreateQuestion(ctx, q); err != nil {
				log.Error("Failed to seed question: "+q.PublicID, err)
			} else {
				log.Info("Seeded question: " + q.PublicID)
			}
		}
	}

	log.Info(fmt.Sprintf("Database seeding completed successfully! Total questions: %d", len(sampleQuestions)))
}
