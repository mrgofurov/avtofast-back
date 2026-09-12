package usecase

import (
	"context"
	"errors"

	"github.com/avtofast/avtofast-back/internal/domain"
	"github.com/avtofast/avtofast-back/pkg/jwt"
)

type AuthUsecase struct {
	userRepo domain.UserRepository
	verifier jwt.TokenVerifier
}

func NewAuthUsecase(userRepo domain.UserRepository, verifier jwt.TokenVerifier) *AuthUsecase {
	return &AuthUsecase{
		userRepo: userRepo,
		verifier: verifier,
	}
}

func (u *AuthUsecase) Authenticate(ctx context.Context, tokenStr string) (*domain.User, error) {
	claims, err := u.verifier.Verify(tokenStr)
	if err != nil {
		return nil, err
	}

	user, err := u.userRepo.GetOrCreateByProvider(
		ctx,
		claims.Provider,
		claims.Subject,
		claims.Email,
		claims.Phone,
		claims.Email, // default display name to email or sub
	)
	if err != nil {
		return nil, err
	}

	// Override role if token specifies admin role
	if claims.Role != "" && claims.Role != user.Role && claims.Role != domain.RoleUser {
		user.Role = claims.Role
		_ = u.userRepo.Update(ctx, user)
	}

	return user, nil
}

type ProfileUsecase struct {
	userRepo domain.UserRepository
	entRepo  domain.EntitlementRepository
}

func NewProfileUsecase(userRepo domain.UserRepository, entRepo domain.EntitlementRepository) *ProfileUsecase {
	return &ProfileUsecase{
		userRepo: userRepo,
		entRepo:  entRepo,
	}
}

type MeResponse struct {
	ID                  string                  `json:"id"`
	DisplayName         string                  `json:"displayName"`
	AvatarURL           *string                 `json:"avatarUrl"`
	OnboardingCompleted bool                    `json:"onboardingCompleted"`
	Onboarding          *domain.UserOnboarding  `json:"onboarding,omitempty"`
	Preferences         *domain.UserPreferences `json:"preferences,omitempty"`
	Entitlement         struct {
		Tier      string  `json:"tier"`
		ExpiresAt *string `json:"expiresAt"`
	} `json:"entitlement"`
}

func (u *ProfileUsecase) GetProfile(ctx context.Context, userID int64) (*MeResponse, error) {
	user, err := u.userRepo.GetByID(ctx, userID)
	if err != nil || user == nil {
		return nil, errors.New("user not found")
	}

	onboarding, _ := u.userRepo.GetOnboarding(ctx, userID)
	prefs, _ := u.userRepo.GetPreferences(ctx, userID)
	ent, _ := u.entRepo.GetByUserID(ctx, userID)

	res := &MeResponse{
		ID:                  user.PublicID,
		DisplayName:         user.DisplayName,
		AvatarURL:           user.AvatarURL,
		OnboardingCompleted: onboarding != nil && onboarding.Completed,
		Onboarding:          onboarding,
		Preferences:         prefs,
	}

	if ent != nil {
		res.Entitlement.Tier = ent.Tier
		if ent.ExpiresAt != nil {
			str := ent.ExpiresAt.Format("2006-01-02T15:04:05Z07:00")
			res.Entitlement.ExpiresAt = &str
		}
	} else {
		res.Entitlement.Tier = domain.TierFree
	}

	return res, nil
}

func (u *ProfileUsecase) UpdateProfile(ctx context.Context, userID int64, displayName *string, avatarURL *string) error {
	user, err := u.userRepo.GetByID(ctx, userID)
	if err != nil || user == nil {
		return errors.New("user not found")
	}
	if displayName != nil {
		user.DisplayName = *displayName
	}
	if avatarURL != nil {
		user.AvatarURL = avatarURL
	}
	return u.userRepo.Update(ctx, user)
}

func (u *ProfileUsecase) SaveOnboarding(ctx context.Context, userID int64, ob *domain.UserOnboarding) error {
	if ob.DailyQuestionGoal < 1 || ob.DailyQuestionGoal > 100 {
		return errors.New("dailyQuestionGoal must be between 1 and 100")
	}
	ob.UserID = userID
	return u.userRepo.SaveOnboarding(ctx, ob)
}

func (u *ProfileUsecase) UpdatePreferences(ctx context.Context, userID int64, prefs *domain.UserPreferences) error {
	prefs.UserID = userID
	return u.userRepo.UpdatePreferences(ctx, prefs)
}

func (u *ProfileUsecase) GetNotificationPreferences(ctx context.Context, userID int64) (*domain.NotificationPreferences, error) {
	return u.userRepo.GetNotificationPreferences(ctx, userID)
}

func (u *ProfileUsecase) UpdateNotificationPreferences(ctx context.Context, userID int64, prefs *domain.NotificationPreferences) error {
	prefs.UserID = userID
	return u.userRepo.UpdateNotificationPreferences(ctx, prefs)
}
