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
)

type ContentUsecase struct {
	contentRepo domain.ContentRepository
	entRepo     domain.EntitlementRepository
	cfg         *config.Config
}

func NewContentUsecase(contentRepo domain.ContentRepository, entRepo domain.EntitlementRepository, cfg *config.Config) *ContentUsecase {
	return &ContentUsecase{
		contentRepo: contentRepo,
		entRepo:     entRepo,
		cfg:         cfg,
	}
}

type BootstrapResponse struct {
	MinimumSupportedAppVersion string          `json:"minimumSupportedAppVersion"`
	DefaultLocale              string          `json:"defaultLocale"`
	SupportedLocales           []string        `json:"supportedLocales"`
	ExamRules                  ExamRulesConfig `json:"examRules"`
	ActiveQuestionPack         *ActivePackInfo `json:"activeQuestionPack"`
	Features                   map[string]bool `json:"features"`
}

type ExamRulesConfig struct {
	QuestionCount    int `json:"questionCount"`
	DurationSeconds  int `json:"durationSeconds"`
	PassCorrectCount int `json:"passCorrectCount"`
}

type ActivePackInfo struct {
	ID          string  `json:"id"`
	Version     string  `json:"version"`
	PublishedAt *string `json:"publishedAt"`
}

func (u *ContentUsecase) GetBootstrap(ctx context.Context) (*BootstrapResponse, error) {
	activePack, _ := u.contentRepo.GetActivePack(ctx)

	res := &BootstrapResponse{
		MinimumSupportedAppVersion: "1.0.0",
		DefaultLocale:              domain.LocaleUzLatn,
		SupportedLocales:           []string{domain.LocaleUzLatn, domain.LocaleUzCyrl, domain.LocaleRu, domain.LocaleEn},
		ExamRules: ExamRulesConfig{
			QuestionCount:    u.cfg.Rules.ExamQuestionCount,
			DurationSeconds:  u.cfg.Rules.ExamDurationSec,
			PassCorrectCount: u.cfg.Rules.ExamPassCount,
		},
		Features: map[string]bool{
			"friends":           u.cfg.Features.Friends,
			"weeklyChallenges":  u.cfg.Features.WeeklyChallenges,
			"voiceExplanations": u.cfg.Features.VoiceExplanations,
			"aiMistakeAnalysis": u.cfg.Features.AIMistakeAnalysis,
		},
	}

	if activePack != nil {
		var pubAt *string
		if activePack.PublishedAt != nil {
			str := activePack.PublishedAt.Format("2006-01-02T15:04:05Z07:00")
			pubAt = &str
		}
		res.ActiveQuestionPack = &ActivePackInfo{
			ID:          activePack.PackID,
			Version:     activePack.Version,
			PublishedAt: pubAt,
		}
	}

	return res, nil
}

type PackCatalogueItem struct {
	ID              string   `json:"id"`
	Version         string   `json:"version"`
	Title           string   `json:"title"`
	Locales         []string `json:"locales"`
	QuestionCount   int      `json:"questionCount"`
	DownloadBytes   int64    `json:"downloadBytes"`
	MandatoryUpdate bool     `json:"mandatoryUpdate"`
}

func (u *ContentUsecase) GetPacks(ctx context.Context) ([]PackCatalogueItem, error) {
	packs, err := u.contentRepo.GetAllPacks(ctx)
	if err != nil {
		return nil, err
	}

	var items []PackCatalogueItem
	for _, p := range packs {
		items = append(items, PackCatalogueItem{
			ID:              p.PackID,
			Version:         p.Version,
			Title:           p.Title,
			Locales:         p.Locales,
			QuestionCount:   p.QuestionCount,
			DownloadBytes:   p.DownloadBytes,
			MandatoryUpdate: p.MandatoryUpdate,
		})
	}
	return items, nil
}

type PackManifest struct {
	PackID    string `json:"packId"`
	Version   string `json:"version"`
	SHA256    string `json:"sha256"`
	Signature string `json:"signature"`
}

type OfflineDownloadResponse struct {
	URL       string       `json:"url"`
	ExpiresAt string       `json:"expiresAt"`
	Manifest  PackManifest `json:"manifest"`
}

func (u *ContentUsecase) GetPackManifest(ctx context.Context, packID string) (*PackManifest, error) {
	pack, err := u.contentRepo.GetPackByID(ctx, packID)
	if err != nil || pack == nil {
		return nil, errors.New("question pack not found")
	}

	manifestPayload := fmt.Sprintf("%s:%s:%s", pack.PackID, pack.Version, pack.ManifestSHA256)
	signature := pack.ManifestSignature
	if signature == "" && len(u.cfg.Ed25519PrivKey) == ed25519.PrivateKeySize {
		signature = crypto.SignEd25519(u.cfg.Ed25519PrivKey, []byte(manifestPayload))
	}

	return &PackManifest{
		PackID:    pack.PackID,
		Version:   pack.Version,
		SHA256:    pack.ManifestSHA256,
		Signature: signature,
	}, nil
}

func (u *ContentUsecase) GetQuestions(ctx context.Context, packID, locale, category, cursor string, limit int) ([]domain.ClientQuestionPayload, string, error) {
	if locale == "" {
		locale = domain.LocaleUzLatn
	}

	questions, nextCursor, err := u.contentRepo.GetQuestionsByPack(ctx, packID, category, cursor, limit)
	if err != nil {
		return nil, "", err
	}

	var payloads []domain.ClientQuestionPayload
	for _, q := range questions {
		payloads = append(payloads, q.ToClientPayload(locale))
	}

	return payloads, nextCursor, nil
}

func (u *ContentUsecase) GetOfflineDownloadURL(ctx context.Context, userID int64, packID string) (*OfflineDownloadResponse, error) {
	manifest, err := u.GetPackManifest(ctx, packID)
	if err != nil {
		return nil, err
	}

	expiresAt := time.Now().UTC().Add(1 * time.Hour)
	downloadURL := fmt.Sprintf("https://api.avtofast.uz/v1/content/packs/%s/download?version=%s&expires=%d", packID, manifest.Version, expiresAt.Unix())

	return &OfflineDownloadResponse{
		URL:       downloadURL,
		ExpiresAt: expiresAt.Format(time.RFC3339),
		Manifest:  *manifest,
	}, nil
}
