package usecase

import (
	"context"
	"crypto/ed25519"
	"errors"
	"fmt"
	"time"

	"github.com/avtofast/avtofast-back/internal/config"
	"github.com/avtofast/avtofast-back/internal/domain"
	"github.com/avtofast/avtofast-back/pkg/crypto"
	"github.com/avtofast/avtofast-back/pkg/id"
)

type AdminUsecase struct {
	contentRepo domain.ContentRepository
	auditRepo   domain.AuditRepository
	cfg         *config.Config
}

func NewAdminUsecase(contentRepo domain.ContentRepository, auditRepo domain.AuditRepository, cfg *config.Config) *AdminUsecase {
	return &AdminUsecase{
		contentRepo: contentRepo,
		auditRepo:   auditRepo,
		cfg:         cfg,
	}
}

type CreatePackDraftRequest struct {
	PackID  string   `json:"id"`
	Version string   `json:"version"`
	Title   string   `json:"title"`
	Locales []string `json:"locales"`
}

func (u *AdminUsecase) CreateDraftPack(ctx context.Context, actorID int64, req CreatePackDraftRequest) (*domain.QuestionPack, error) {
	if len(req.Locales) == 0 {
		req.Locales = []string{domain.LocaleUzLatn, domain.LocaleUzCyrl, domain.LocaleRu, domain.LocaleEn}
	}

	pack := &domain.QuestionPack{
		PackID:        req.PackID,
		Version:       req.Version,
		Title:         req.Title,
		Locales:       req.Locales,
		Status:        "draft",
		QuestionCount: 0,
	}

	if err := u.contentRepo.CreatePack(ctx, pack); err != nil {
		return nil, err
	}

	_ = u.auditRepo.LogAction(ctx, &domain.AuditLog{
		PublicID:   id.New(id.PrefixAudit),
		ActorID:    actorID,
		Action:     "create_pack_draft",
		TargetType: "question_pack",
		TargetID:   pack.PackID,
		AfterState: pack,
		Reason:     "Created new pack draft",
	})

	return pack, nil
}

func (u *AdminUsecase) AddQuestion(ctx context.Context, actorID int64, packID string, q *domain.Question) error {
	pack, err := u.contentRepo.GetPackByID(ctx, packID)
	if err != nil || pack == nil {
		return errors.New("question pack not found")
	}

	q.PackTableID = pack.ID
	q.PackID = packID
	q.ContentVersion = pack.Version
	q.Status = "draft"

	if err := u.contentRepo.CreateQuestion(ctx, q); err != nil {
		return err
	}

	pack.QuestionCount++
	_ = u.contentRepo.UpdatePack(ctx, pack)

	_ = u.auditRepo.LogAction(ctx, &domain.AuditLog{
		PublicID:   id.New(id.PrefixAudit),
		ActorID:    actorID,
		Action:     "add_draft_question",
		TargetType: "question",
		TargetID:   q.PublicID,
		AfterState: q,
		Reason:     "Added draft question to pack",
	})

	return nil
}

type QuestionValidationResult struct {
	Valid  bool     `json:"valid"`
	Errors []string `json:"errors,omitempty"`
}

func (u *AdminUsecase) ValidateQuestion(ctx context.Context, questionID string) (*QuestionValidationResult, error) {
	q, err := u.contentRepo.GetQuestionByID(ctx, questionID)
	if err != nil || q == nil {
		return nil, errors.New("question not found")
	}

	var errs []string
	requiredLocales := []string{domain.LocaleUzLatn, domain.LocaleUzCyrl, domain.LocaleRu, domain.LocaleEn}

	for _, loc := range requiredLocales {
		tr, ok := q.Translations[loc]
		if !ok {
			errs = append(errs, fmt.Sprintf("missing translation for locale: %s", loc))
			continue
		}
		if tr.Prompt == "" {
			errs = append(errs, fmt.Sprintf("prompt is empty for locale: %s", loc))
		}
		if len(tr.Choices) < 2 {
			errs = append(errs, fmt.Sprintf("insufficient choices (<2) for locale: %s", loc))
		}
		if tr.Explanation == "" {
			errs = append(errs, fmt.Sprintf("explanation is empty for locale: %s", loc))
		}
	}

	if q.CorrectChoiceID == "" {
		errs = append(errs, "correctChoiceId must not be empty")
	}

	if q.Image != nil && q.Image.SHA256 == "" {
		errs = append(errs, "image sha256 checksum is missing")
	}

	return &QuestionValidationResult{
		Valid:  len(errs) == 0,
		Errors: errs,
	}, nil
}

func (u *AdminUsecase) PublishPack(ctx context.Context, actorID int64, packID string, reason string) (*domain.QuestionPack, error) {
	pack, err := u.contentRepo.GetPackByID(ctx, packID)
	if err != nil || pack == nil {
		return nil, errors.New("question pack not found")
	}

	now := time.Now().UTC()
	manifestPayload := fmt.Sprintf("%s:%s:%s", pack.PackID, pack.Version, now.Format(time.RFC3339))
	sha := crypto.SHA256Hex([]byte(manifestPayload))

	sig := ""
	if len(u.cfg.Ed25519PrivKey) == ed25519.PrivateKeySize {
		sig = crypto.SignEd25519(u.cfg.Ed25519PrivKey, []byte(sha))
	}

	beforeState := *pack
	pack.Status = "published"
	pack.ManifestSHA256 = sha
	pack.ManifestSignature = sig
	pack.PublishedAt = &now

	if err := u.contentRepo.UpdatePack(ctx, pack); err != nil {
		return nil, err
	}

	_ = u.auditRepo.LogAction(ctx, &domain.AuditLog{
		PublicID:    id.New(id.PrefixAudit),
		ActorID:     actorID,
		Action:      "publish_pack",
		TargetType:  "question_pack",
		TargetID:    pack.PackID,
		BeforeState: beforeState,
		AfterState:  pack,
		Reason:      reason,
	})

	return pack, nil
}

func (u *AdminUsecase) RollbackPack(ctx context.Context, actorID int64, packID, targetVersion string, reason string) error {
	pack, err := u.contentRepo.GetPackByID(ctx, packID)
	if err != nil || pack == nil {
		return errors.New("question pack not found")
	}

	_ = u.auditRepo.LogAction(ctx, &domain.AuditLog{
		PublicID:   id.New(id.PrefixAudit),
		ActorID:    actorID,
		Action:     "rollback_pack",
		TargetType: "question_pack",
		TargetID:   pack.PackID,
		Reason:     reason,
	})

	return nil
}

func (u *AdminUsecase) GetAuditLogs(ctx context.Context, cursor string, limit int) ([]*domain.AuditLog, string, error) {
	return u.auditRepo.GetAuditLogs(ctx, cursor, limit)
}
