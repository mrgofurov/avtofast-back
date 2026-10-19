package postgres

import (
	"context"
	"encoding/json"
	"regexp"
	"strconv"
	"strings"

	"github.com/avtofast/avtofast-back/internal/domain"
)

// RoadSignRepository serves the road sign reference and fills it on import.
type RoadSignRepository struct {
	db          *DB
	contentRepo *ContentRepository
}

func NewRoadSignRepository(db *DB, contentRepo *ContentRepository) *RoadSignRepository {
	return &RoadSignRepository{db: db, contentRepo: contentRepo}
}

func (r *RoadSignRepository) ListSigns(ctx context.Context, locale string) ([]domain.RoadSignGroup, error) {
	descriptions, err := r.signDescriptions(ctx)
	if err != nil {
		return nil, err
	}

	rows, err := r.db.Pool.Query(ctx, `SELECT s.code, s.group_key, s.group_title, s.title, s.image_url,
	                                         (SELECT COUNT(*) FROM road_sign_questions sq
	                                            JOIN questions q ON q.id = sq.question_id AND q.status = 'published'
	                                           WHERE sq.sign_id = s.id)
	                                  FROM road_signs s
	                                  ORDER BY s.group_order, s.ordinal`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var groups []domain.RoadSignGroup
	for rows.Next() {
		var sign domain.RoadSign
		var groupKey, groupTitle string
		var titleRaw []byte
		if err := rows.Scan(&sign.Code, &groupKey, &groupTitle, &titleRaw, &sign.ImageURL, &sign.QuestionCount); err != nil {
			return nil, err
		}
		var titles map[string]string
		_ = json.Unmarshal(titleRaw, &titles)
		sign.Title = titles[locale]
		if sign.Title == "" {
			sign.Title = titles[domain.LocaleUzLatn]
		}
		sign.ImageURL = domain.PublicMediaPath(sign.ImageURL)
		sign.Description = describe(descriptions, sign.Code)
		if sign.Description == nil {
			sign.Description = []string{}
		}

		if len(groups) == 0 || groups[len(groups)-1].Key != groupKey {
			groups = append(groups, domain.RoadSignGroup{Key: groupKey, Title: groupTitle})
		}
		g := &groups[len(groups)-1]
		g.Signs = append(g.Signs, sign)
	}
	return groups, rows.Err()
}

// signDescriptions maps each sign number to its paragraphs in appendix 1 of
// the Rules, in order. A paragraph headed by a range ("1.4.1 — 1.4.6") belongs
// to every sign in it.
func (r *RoadSignRepository) signDescriptions(ctx context.Context) (map[string][]string, error) {
	rows, err := r.db.Pool.Query(ctx, `SELECT clause_number, text FROM legal_paragraphs
	                                  WHERE document = 'yhq' AND appendix LIKE '1-ilova%'
	                                  ORDER BY ordinal`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := map[string][]string{}
	for rows.Next() {
		var clause, text string
		if err := rows.Scan(&clause, &text); err != nil {
			return nil, err
		}
		if clause == "" {
			// A few entries reached the import without their number split
			// off ("4.9.1, 4.9.2 va 4.9.3 “…”"); read it from the text.
			clause = leadingCodes.FindString(text)
		}
		for _, code := range expandSignCodes(clause) {
			out[code] = append(out[code], text)
		}
	}
	return out, rows.Err()
}

// describe finds a sign's wording even where the source numbers it at a
// different depth than the Rules do: "7.21.1" falls back to the Rules' 7.21,
// and "6.3" to its parts 6.3.1, 6.3.2, … in order.
func describe(descriptions map[string][]string, code string) []string {
	if code == "" {
		return nil
	}
	if d, ok := descriptions[code]; ok {
		return d
	}
	if i := strings.LastIndex(code, "."); i > 0 {
		if d, ok := descriptions[code[:i]]; ok && strings.Count(code, ".") > 1 {
			return d
		}
	}
	var parts []string
	for n := 1; n < 20; n++ {
		d, ok := descriptions[code+"."+strconv.Itoa(n)]
		if !ok {
			break
		}
		parts = append(parts, d...)
	}
	return parts
}

var leadingCodes = regexp.MustCompile(`^\d+(?:\.\d+)+(?:\s*(?:,|va)\s*\d+(?:\.\d+)+)*`)

var signRange = regexp.MustCompile(`^(\d+(?:\.\d+)*)\.(\d+)\s*[—–-]\s*(\d+(?:\.\d+)*)\.(\d+)$`)

// expandSignCodes turns a clause heading into the sign numbers it covers:
// "2.4" → [2.4]; "1.4.1 — 1.4.6" → [1.4.1 … 1.4.6]; "5.15.1, 5.15.2" → both.
func expandSignCodes(clause string) []string {
	var out []string
	for _, part := range strings.Split(strings.ReplaceAll(clause, " va ", ","), ",") {
		part = strings.TrimSpace(strings.TrimSuffix(strings.TrimSpace(part), "."))
		if m := signRange.FindStringSubmatch(part); m != nil && m[1] == m[3] {
			from, _ := strconv.Atoi(m[2])
			to, _ := strconv.Atoi(m[4])
			if from <= to && to-from < 50 {
				for i := from; i <= to; i++ {
					out = append(out, m[1]+"."+strconv.Itoa(i))
				}
				continue
			}
		}
		if part != "" {
			out = append(out, part)
		}
	}
	return out
}

func (r *RoadSignRepository) SignQuestions(ctx context.Context, code string) ([]*domain.Question, error) {
	rows, err := r.db.Pool.Query(ctx, `SELECT DISTINCT q.public_id, q.id
	                                  FROM road_signs s
	                                  JOIN road_sign_questions sq ON sq.sign_id = s.id
	                                  JOIN questions q ON q.id = sq.question_id
	                                  WHERE s.code = $1 AND q.status = 'published'
	                                  ORDER BY q.id`, code)
	if err != nil {
		return nil, err
	}
	var ids []string
	for rows.Next() {
		var id string
		var seq int64
		if err := rows.Scan(&id, &seq); err != nil {
			rows.Close()
			return nil, err
		}
		ids = append(ids, id)
	}
	rows.Close()
	if err := rows.Err(); err != nil || len(ids) == 0 {
		return nil, err
	}
	loaded, err := r.contentRepo.GetQuestionsByIDs(ctx, ids)
	if err != nil {
		return nil, err
	}
	byID := make(map[string]*domain.Question, len(loaded))
	for _, q := range loaded {
		byID[q.PublicID] = q
	}
	ordered := make([]*domain.Question, 0, len(ids))
	for _, id := range ids {
		if q, ok := byID[id]; ok {
			ordered = append(ordered, q)
		}
	}
	return ordered, nil
}

// RoadSignImport is one sign to store.
type RoadSignImport struct {
	Code       string
	GroupKey   string
	GroupTitle string
	GroupOrder int
	Ordinal    int
	Titles     map[string]string
	ImageURL   string
	Source     string
	// Public ids of the questions that cite the sign.
	QuestionIDs []string
}

// ReplaceSigns replaces the whole reference with the signs given, all or none.
func (r *RoadSignRepository) ReplaceSigns(ctx context.Context, signs []RoadSignImport) error {
	tx, err := r.db.Pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	if _, err := tx.Exec(ctx, `DELETE FROM road_signs`); err != nil {
		return err
	}
	for _, s := range signs {
		titles, _ := json.Marshal(s.Titles)
		var signID int64
		err := tx.QueryRow(ctx, `INSERT INTO road_signs (code, group_key, group_title, group_order, ordinal, title, image_url, source)
		                         VALUES ($1, $2, $3, $4, $5, $6, $7, $8) RETURNING id`,
			s.Code, s.GroupKey, s.GroupTitle, s.GroupOrder, s.Ordinal, titles, s.ImageURL, s.Source).Scan(&signID)
		if err != nil {
			return err
		}
		if len(s.QuestionIDs) > 0 {
			if _, err := tx.Exec(ctx, `INSERT INTO road_sign_questions (sign_id, question_id)
			                           SELECT $1, id FROM questions WHERE public_id = ANY($2)
			                           ON CONFLICT DO NOTHING`, signID, s.QuestionIDs); err != nil {
				return err
			}
		}
	}
	return tx.Commit(ctx)
}
