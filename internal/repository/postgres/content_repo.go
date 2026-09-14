package postgres

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/avtofast/avtofast-back/internal/domain"
	"github.com/jackc/pgx/v5"
)

type ContentRepository struct {
	db *DB
}

func NewContentRepository(db *DB) *ContentRepository {
	return &ContentRepository{db: db}
}

func (r *ContentRepository) GetActivePack(ctx context.Context) (*domain.QuestionPack, error) {
	query := `SELECT id, pack_id, version, title, locales, question_count, download_bytes, mandatory_update, status, manifest_sha256, manifest_signature, published_at
	          FROM question_packs WHERE status = 'published' ORDER BY published_at DESC LIMIT 1`
	var p domain.QuestionPack
	err := r.db.Pool.QueryRow(ctx, query).Scan(
		&p.ID, &p.PackID, &p.Version, &p.Title, &p.Locales, &p.QuestionCount,
		&p.DownloadBytes, &p.MandatoryUpdate, &p.Status, &p.ManifestSHA256, &p.ManifestSignature, &p.PublishedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	return &p, nil
}

func (r *ContentRepository) GetAllPacks(ctx context.Context) ([]*domain.QuestionPack, error) {
	query := `SELECT id, pack_id, version, title, locales, question_count, download_bytes, mandatory_update, status, manifest_sha256, manifest_signature, published_at
	          FROM question_packs WHERE status = 'published' ORDER BY published_at DESC`
	rows, err := r.db.Pool.Query(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var packs []*domain.QuestionPack
	for rows.Next() {
		var p domain.QuestionPack
		if err := rows.Scan(
			&p.ID, &p.PackID, &p.Version, &p.Title, &p.Locales, &p.QuestionCount,
			&p.DownloadBytes, &p.MandatoryUpdate, &p.Status, &p.ManifestSHA256, &p.ManifestSignature, &p.PublishedAt,
		); err != nil {
			return nil, err
		}
		packs = append(packs, &p)
	}
	return packs, nil
}

func (r *ContentRepository) GetPackByID(ctx context.Context, packID string) (*domain.QuestionPack, error) {
	query := `SELECT id, pack_id, version, title, locales, question_count, download_bytes, mandatory_update, status, manifest_sha256, manifest_signature, published_at
	          FROM question_packs WHERE pack_id = $1 ORDER BY version DESC LIMIT 1`
	var p domain.QuestionPack
	err := r.db.Pool.QueryRow(ctx, query, packID).Scan(
		&p.ID, &p.PackID, &p.Version, &p.Title, &p.Locales, &p.QuestionCount,
		&p.DownloadBytes, &p.MandatoryUpdate, &p.Status, &p.ManifestSHA256, &p.ManifestSignature, &p.PublishedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	return &p, nil
}

func (r *ContentRepository) loadTranslations(ctx context.Context, qIDs []int64) (map[int64]map[string]domain.QuestionTranslationData, error) {
	result := make(map[int64]map[string]domain.QuestionTranslationData)
	if len(qIDs) == 0 {
		return result, nil
	}

	query := `SELECT question_id, locale, prompt, choices, explanation FROM question_translations WHERE question_id = ANY($1)`
	rows, err := r.db.Pool.Query(ctx, query, qIDs)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		var qID int64
		var locale, prompt, explanation string
		var choicesRaw []byte
		if err := rows.Scan(&qID, &locale, &prompt, &choicesRaw, &explanation); err != nil {
			return nil, err
		}
		var choices []domain.ChoiceItem
		_ = json.Unmarshal(choicesRaw, &choices)

		if result[qID] == nil {
			result[qID] = make(map[string]domain.QuestionTranslationData)
		}
		result[qID][locale] = domain.QuestionTranslationData{
			Prompt:      prompt,
			Choices:     choices,
			Explanation: explanation,
		}
	}
	return result, nil
}

func (r *ContentRepository) GetQuestionsByPack(ctx context.Context, packID, category string, cursor string, limit int) ([]*domain.Question, string, error) {
	if limit <= 0 || limit > 100 {
		limit = 20
	}

	whereClause := "WHERE q.pack_id = $1 AND q.status = 'published'"
	args := []any{packID}
	argIdx := 2

	if category != "" {
		whereClause += fmt.Sprintf(" AND q.category = $%d", argIdx)
		args = append(args, category)
		argIdx++
	}

	if cursor != "" {
		whereClause += fmt.Sprintf(" AND q.public_id > $%d", argIdx)
		args = append(args, cursor)
		argIdx++
	}

	query := fmt.Sprintf(`SELECT q.id, q.public_id, q.pack_table_id, q.pack_id, q.content_version, q.category, q.difficulty,
	                             q.image_url, q.image_sha256, q.image_alt, q.video_url, q.audio_url, q.external_id,
	                             q.source_reference, q.source_effective_from,
	                             q.source_official_url, q.correct_choice_id, q.status
	                      FROM questions q %s ORDER BY q.public_id ASC LIMIT $%d`, whereClause, argIdx)
	args = append(args, limit+1)

	rows, err := r.db.Pool.Query(ctx, query, args...)
	if err != nil {
		return nil, "", err
	}
	defer rows.Close()

	var questions []*domain.Question
	var qIDs []int64

	for rows.Next() {
		var q domain.Question
		var imgURL, imgSHA, srcRef, srcOff *string
		var videoURL, audioURL *string
		var extID *int64
		var srcEff *time.Time
		var imgAltRaw []byte

		if err := rows.Scan(
			&q.ID, &q.PublicID, &q.PackTableID, &q.PackID, &q.ContentVersion, &q.Category, &q.Difficulty,
			&imgURL, &imgSHA, &imgAltRaw, &videoURL, &audioURL, &extID,
			&srcRef, &srcEff, &srcOff, &q.CorrectChoiceID, &q.Status,
		); err != nil {
			return nil, "", err
		}

		if videoURL != nil {
			q.VideoURL = *videoURL
		}
		if audioURL != nil {
			q.AudioURL = *audioURL
		}
		if extID != nil {
			q.ExternalID = *extID
		}

		if imgURL != nil && *imgURL != "" {
			var alt map[string]string
			_ = json.Unmarshal(imgAltRaw, &alt)
			q.Image = &domain.QuestionImage{
				URL:    *imgURL,
				SHA256: *imgSHA,
				Alt:    alt,
			}
		}

		q.Source = &domain.QuestionSource{}
		if srcRef != nil {
			q.Source.Reference = *srcRef
		}
		if srcEff != nil {
			q.Source.EffectiveFrom = srcEff.Format("2006-01-02")
		}
		if srcOff != nil {
			q.Source.OfficialSourceURL = *srcOff
		}

		questions = append(questions, &q)
		qIDs = append(qIDs, q.ID)
	}

	var nextCursor string
	if len(questions) > limit {
		nextCursor = questions[limit-1].PublicID
		questions = questions[:limit]
		qIDs = qIDs[:limit]
	}

	// Load translations
	transMap, err := r.loadTranslations(ctx, qIDs)
	if err != nil {
		return nil, "", err
	}

	for _, q := range questions {
		q.Translations = transMap[q.ID]
	}

	return questions, nextCursor, nil
}

func (r *ContentRepository) GetQuestionByID(ctx context.Context, questionPublicID string) (*domain.Question, error) {
	query := `SELECT q.id, q.public_id, q.pack_table_id, q.pack_id, q.content_version, q.category, q.difficulty,
	                 q.image_url, q.image_sha256, q.image_alt, q.video_url, q.audio_url, q.external_id,
	                 q.source_reference, q.source_effective_from,
	                 q.source_official_url, q.correct_choice_id, q.status
	          FROM questions q WHERE q.public_id = $1`
	var q domain.Question
	var imgURL, imgSHA, srcRef, srcOff *string
	var videoURL, audioURL *string
	var extID *int64
	var srcEff *time.Time
	var imgAltRaw []byte

	err := r.db.Pool.QueryRow(ctx, query, questionPublicID).Scan(
		&q.ID, &q.PublicID, &q.PackTableID, &q.PackID, &q.ContentVersion, &q.Category, &q.Difficulty,
		&imgURL, &imgSHA, &imgAltRaw, &videoURL, &audioURL, &extID,
		&srcRef, &srcEff, &srcOff, &q.CorrectChoiceID, &q.Status,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}

	if videoURL != nil {
		q.VideoURL = *videoURL
	}
	if audioURL != nil {
		q.AudioURL = *audioURL
	}
	if extID != nil {
		q.ExternalID = *extID
	}

	if imgURL != nil && *imgURL != "" {
		var alt map[string]string
		_ = json.Unmarshal(imgAltRaw, &alt)
		q.Image = &domain.QuestionImage{
			URL:    *imgURL,
			SHA256: *imgSHA,
			Alt:    alt,
		}
	}
	q.Source = &domain.QuestionSource{}
	if srcRef != nil {
		q.Source.Reference = *srcRef
	}
	if srcEff != nil {
		q.Source.EffectiveFrom = srcEff.Format("2006-01-02")
	}
	if srcOff != nil {
		q.Source.OfficialSourceURL = *srcOff
	}

	transMap, err := r.loadTranslations(ctx, []int64{q.ID})
	if err != nil {
		return nil, err
	}
	q.Translations = transMap[q.ID]

	return &q, nil
}

func (r *ContentRepository) GetQuestionsByIDs(ctx context.Context, questionPublicIDs []string) ([]*domain.Question, error) {
	if len(questionPublicIDs) == 0 {
		return nil, nil
	}
	query := `SELECT q.id, q.public_id, q.pack_table_id, q.pack_id, q.content_version, q.category, q.difficulty,
	                 q.image_url, q.image_sha256, q.image_alt, q.video_url, q.audio_url, q.external_id,
	                 q.source_reference, q.source_effective_from,
	                 q.source_official_url, q.correct_choice_id, q.status
	          FROM questions q WHERE q.public_id = ANY($1)`
	rows, err := r.db.Pool.Query(ctx, query, questionPublicIDs)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var questions []*domain.Question
	var qIDs []int64
	for rows.Next() {
		var q domain.Question
		var imgURL, imgSHA, srcRef, srcOff *string
		var videoURL, audioURL *string
		var extID *int64
		var srcEff *time.Time
		var imgAltRaw []byte

		if err := rows.Scan(
			&q.ID, &q.PublicID, &q.PackTableID, &q.PackID, &q.ContentVersion, &q.Category, &q.Difficulty,
			&imgURL, &imgSHA, &imgAltRaw, &videoURL, &audioURL, &extID,
			&srcRef, &srcEff, &srcOff, &q.CorrectChoiceID, &q.Status,
		); err != nil {
			return nil, err
		}
		if videoURL != nil {
			q.VideoURL = *videoURL
		}
		if audioURL != nil {
			q.AudioURL = *audioURL
		}
		if extID != nil {
			q.ExternalID = *extID
		}
		if imgURL != nil && *imgURL != "" {
			var alt map[string]string
			_ = json.Unmarshal(imgAltRaw, &alt)
			q.Image = &domain.QuestionImage{
				URL:    *imgURL,
				SHA256: *imgSHA,
				Alt:    alt,
			}
		}
		q.Source = &domain.QuestionSource{}
		if srcRef != nil {
			q.Source.Reference = *srcRef
		}
		if srcEff != nil {
			q.Source.EffectiveFrom = srcEff.Format("2006-01-02")
		}
		if srcOff != nil {
			q.Source.OfficialSourceURL = *srcOff
		}
		questions = append(questions, &q)
		qIDs = append(qIDs, q.ID)
	}

	transMap, err := r.loadTranslations(ctx, qIDs)
	if err != nil {
		return nil, err
	}
	for _, q := range questions {
		q.Translations = transMap[q.ID]
	}
	return questions, nil
}

func (r *ContentRepository) GetRandomQuestions(ctx context.Context, packID string, count int, category string) ([]*domain.Question, error) {
	where := "WHERE pack_id = $1 AND status = 'published'"
	args := []any{packID}
	if category != "" {
		where += " AND category = $2"
		args = append(args, category)
	}

	query := fmt.Sprintf(`SELECT id, public_id FROM questions %s ORDER BY RANDOM() LIMIT $%d`, where, len(args)+1)
	args = append(args, count)

	rows, err := r.db.Pool.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var pubIDs []string
	for rows.Next() {
		var id int64
		var pubID string
		if err := rows.Scan(&id, &pubID); err != nil {
			return nil, err
		}
		pubIDs = append(pubIDs, pubID)
	}

	return r.GetQuestionsByIDs(ctx, pubIDs)
}

func (r *ContentRepository) CreatePack(ctx context.Context, pack *domain.QuestionPack) error {
	query := `INSERT INTO question_packs (pack_id, version, title, locales, question_count, download_bytes, mandatory_update, status, manifest_sha256, manifest_signature, published_at, created_at, updated_at)
	          VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, NOW(), NOW())
	          RETURNING id`
	return r.db.Pool.QueryRow(ctx, query,
		pack.PackID, pack.Version, pack.Title, pack.Locales, pack.QuestionCount,
		pack.DownloadBytes, pack.MandatoryUpdate, pack.Status, pack.ManifestSHA256, pack.ManifestSignature, pack.PublishedAt,
	).Scan(&pack.ID)
}

func (r *ContentRepository) UpdatePack(ctx context.Context, pack *domain.QuestionPack) error {
	query := `UPDATE question_packs SET title = $1, locales = $2, question_count = $3, download_bytes = $4,
	          mandatory_update = $5, status = $6, manifest_sha256 = $7, manifest_signature = $8, published_at = $9, updated_at = NOW()
	          WHERE id = $10`
	_, err := r.db.Pool.Exec(ctx, query,
		pack.Title, pack.Locales, pack.QuestionCount, pack.DownloadBytes,
		pack.MandatoryUpdate, pack.Status, pack.ManifestSHA256, pack.ManifestSignature, pack.PublishedAt, pack.ID,
	)
	return err
}

func (r *ContentRepository) CreateQuestion(ctx context.Context, q *domain.Question) error {
	var imgURL, imgSHA *string
	var imgAlt []byte
	if q.Image != nil {
		imgURL = &q.Image.URL
		imgSHA = &q.Image.SHA256
		imgAlt, _ = json.Marshal(q.Image.Alt)
	}

	var srcRef, srcOff *string
	if q.Source != nil {
		srcRef = &q.Source.Reference
		srcOff = &q.Source.OfficialSourceURL
	}

	query := `INSERT INTO questions (public_id, pack_table_id, pack_id, content_version, category, difficulty, image_url, image_sha256, image_alt, video_url, audio_url, external_id, source_reference, source_official_url, correct_choice_id, status, created_at, updated_at)
	          VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, NOW(), NOW())
	          ON CONFLICT (public_id) DO UPDATE SET
	              pack_table_id = EXCLUDED.pack_table_id,
	              pack_id = EXCLUDED.pack_id,
	              content_version = EXCLUDED.content_version,
	              category = EXCLUDED.category,
	              difficulty = EXCLUDED.difficulty,
	              image_url = EXCLUDED.image_url,
	              image_sha256 = EXCLUDED.image_sha256,
	              image_alt = EXCLUDED.image_alt,
	              video_url = EXCLUDED.video_url,
	              audio_url = EXCLUDED.audio_url,
	              external_id = EXCLUDED.external_id,
	              source_reference = EXCLUDED.source_reference,
	              source_official_url = EXCLUDED.source_official_url,
	              correct_choice_id = EXCLUDED.correct_choice_id,
	              status = EXCLUDED.status,
	              updated_at = NOW()
	          RETURNING id`
	var videoURL, audioURL *string
	if q.VideoURL != "" {
		videoURL = &q.VideoURL
	}
	if q.AudioURL != "" {
		audioURL = &q.AudioURL
	}
	var extID *int64
	if q.ExternalID > 0 {
		extID = &q.ExternalID
	}

	err := r.db.Pool.QueryRow(ctx, query,
		q.PublicID, q.PackTableID, q.PackID, q.ContentVersion, q.Category, q.Difficulty,
		imgURL, imgSHA, imgAlt, videoURL, audioURL, extID, srcRef, srcOff, q.CorrectChoiceID, q.Status,
	).Scan(&q.ID)
	if err != nil {
		return err
	}

	// Insert translations
	for loc, tr := range q.Translations {
		choicesBytes, _ := json.Marshal(tr.Choices)
		_, err := r.db.Pool.Exec(ctx,
			`INSERT INTO question_translations (question_id, locale, prompt, choices, explanation, created_at, updated_at)
			 VALUES ($1, $2, $3, $4, $5, NOW(), NOW())
			 ON CONFLICT (question_id, locale) DO UPDATE SET prompt = EXCLUDED.prompt, choices = EXCLUDED.choices, explanation = EXCLUDED.explanation, updated_at = NOW()`,
			q.ID, loc, tr.Prompt, choicesBytes, tr.Explanation,
		)
		if err != nil {
			return err
		}
	}
	return nil
}

func (r *ContentRepository) UpdateQuestion(ctx context.Context, q *domain.Question) error {
	var imgURL, imgSHA *string
	var imgAlt []byte
	if q.Image != nil {
		imgURL = &q.Image.URL
		imgSHA = &q.Image.SHA256
		imgAlt, _ = json.Marshal(q.Image.Alt)
	}

	var srcRef, srcOff *string
	if q.Source != nil {
		srcRef = &q.Source.Reference
		srcOff = &q.Source.OfficialSourceURL
	}

	query := `UPDATE questions SET category = $1, difficulty = $2, image_url = $3, image_sha256 = $4, image_alt = $5,
	          source_reference = $6, source_official_url = $7, correct_choice_id = $8, status = $9, updated_at = NOW()
	          WHERE id = $10`
	_, err := r.db.Pool.Exec(ctx, query,
		q.Category, q.Difficulty, imgURL, imgSHA, imgAlt, srcRef, srcOff, q.CorrectChoiceID, q.Status, q.ID,
	)
	if err != nil {
		return err
	}

	for loc, tr := range q.Translations {
		choicesBytes, _ := json.Marshal(tr.Choices)
		_, err := r.db.Pool.Exec(ctx,
			`INSERT INTO question_translations (question_id, locale, prompt, choices, explanation, created_at, updated_at)
			 VALUES ($1, $2, $3, $4, $5, NOW(), NOW())
			 ON CONFLICT (question_id, locale) DO UPDATE SET prompt = EXCLUDED.prompt, choices = EXCLUDED.choices, explanation = EXCLUDED.explanation, updated_at = NOW()`,
			q.ID, loc, tr.Prompt, choicesBytes, tr.Explanation,
		)
		if err != nil {
			return err
		}
	}
	return nil
}
