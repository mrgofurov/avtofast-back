package usecase

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/avtofast/avtofast-back/internal/domain"
	"github.com/avtofast/avtofast-back/pkg/firebase"
	"github.com/avtofast/avtofast-back/pkg/jwt"
)

var (
	// ErrInvalidIDToken means the Firebase ID token did not verify — expired,
	// tampered with, or minted for another project.
	ErrInvalidIDToken = errors.New("INVALID_ID_TOKEN")
	// ErrInvalidRefreshToken means the refresh token did not verify.
	ErrInvalidRefreshToken = errors.New("INVALID_REFRESH_TOKEN")
	// ErrSignInUnavailable means this deployment has no Firebase project
	// configured, so there is nothing to verify an ID token against.
	ErrSignInUnavailable = errors.New("SIGN_IN_UNAVAILABLE")
	// ErrMissingDeviceID is returned when a guest session is requested with no
	// device to scope it to.
	ErrMissingDeviceID = errors.New("MISSING_DEVICE_ID")
)

// guestProvider is the `provider` column value for a learner who has not
// signed in. Their progress is real and stored server-side; it is simply keyed
// on the installation rather than on a Google/Apple account, and is lost if the
// app is reinstalled. Signing in later creates a separate account — merging the
// two is a product decision, not something to do silently here.
const guestProvider = "guest"

// SessionUsecase issues the API's own session tokens.
//
// Every authenticated endpoint takes an HS256 token signed with the server's
// secret (see pkg/jwt). The mobile client never holds that secret: it arrives
// with either a Firebase ID token from Google/Apple sign-in, or nothing at all
// if the learner declined to sign in. This is the single door both go through.
type SessionUsecase struct {
	userRepo   domain.UserRepository
	firebase   *firebase.Verifier
	issuer     *jwt.Issuer
	verifier   *jwt.Verifier
	accessTTL  time.Duration
	refreshTTL time.Duration
}

func NewSessionUsecase(
	userRepo domain.UserRepository,
	fbVerifier *firebase.Verifier,
	issuer *jwt.Issuer,
	verifier *jwt.Verifier,
	accessTTL, refreshTTL time.Duration,
) *SessionUsecase {
	return &SessionUsecase{
		userRepo:   userRepo,
		firebase:   fbVerifier,
		issuer:     issuer,
		verifier:   verifier,
		accessTTL:  accessTTL,
		refreshTTL: refreshTTL,
	}
}

// SessionResponse is what all three session endpoints return.
type SessionResponse struct {
	AccessToken  string `json:"accessToken"`
	RefreshToken string `json:"refreshToken"`
	TokenType    string `json:"tokenType"`
	// ExpiresIn is the access token's remaining life in seconds, so the client
	// can refresh ahead of expiry instead of waiting for a 401.
	ExpiresIn int          `json:"expiresIn"`
	User      *SessionUser `json:"user"`
}

type SessionUser struct {
	ID          string  `json:"id"`
	DisplayName string  `json:"displayName"`
	Email       string  `json:"email,omitempty"`
	AvatarURL   *string `json:"avatarUrl"`
	Provider    string  `json:"provider"`
	IsGuest     bool    `json:"isGuest"`
}

// ExchangeFirebaseToken trades a verified Firebase ID token for a session,
// creating the user row on first sign-in.
func (u *SessionUsecase) ExchangeFirebaseToken(ctx context.Context, idToken string) (*SessionResponse, error) {
	if !u.firebase.Enabled() {
		return nil, ErrSignInUnavailable
	}

	identity, err := u.firebase.Verify(ctx, idToken)
	if err != nil {
		if errors.Is(err, firebase.ErrKeyUnavailable) {
			return nil, err
		}
		return nil, ErrInvalidIDToken
	}

	displayName := strings.TrimSpace(identity.DisplayName)
	if displayName == "" {
		displayName = identity.Email
	}

	user, err := u.userRepo.GetOrCreateByProvider(
		ctx,
		identity.SignInProvider,
		identity.Subject,
		identity.Email,
		identity.Phone,
		displayName,
	)
	if err != nil {
		return nil, err
	}

	// A learner who edits their Google/Apple profile expects the app to catch
	// up; the ID token is the only place the app learns about it.
	if changed := applyIdentity(user, displayName, identity.AvatarURL); changed {
		_ = u.userRepo.Update(ctx, user)
	}

	return u.issue(user, false)
}

// GuestSession issues a session scoped to one installation, so a learner who
// declines sign-in still gets server-side practice sessions, mistake review
// and a dashboard rather than a read-only demo.
func (u *SessionUsecase) GuestSession(ctx context.Context, deviceID string) (*SessionResponse, error) {
	deviceID = strings.TrimSpace(deviceID)
	if deviceID == "" {
		return nil, ErrMissingDeviceID
	}

	user, err := u.userRepo.GetOrCreateByProvider(ctx, guestProvider, deviceID, "", "", "Guest")
	if err != nil {
		return nil, err
	}
	return u.issue(user, true)
}

// Refresh trades a valid refresh token for a new access token, so a session
// outlives the access token's short life without a second sign-in.
func (u *SessionUsecase) Refresh(ctx context.Context, refreshToken string) (*SessionResponse, error) {
	claims, err := u.verifier.VerifyRefresh(refreshToken)
	if err != nil {
		return nil, ErrInvalidRefreshToken
	}

	user, err := u.userRepo.GetByPublicID(ctx, claims.Subject)
	if err != nil || user == nil {
		return nil, ErrInvalidRefreshToken
	}
	return u.issue(user, user.Provider == guestProvider)
}

func (u *SessionUsecase) issue(user *domain.User, isGuest bool) (*SessionResponse, error) {
	claims := jwt.SessionClaims{
		// The session's subject is our own public id, not the provider's —
		// the token stays valid if the learner relinks a provider, and no
		// third-party identifier leaks into the API surface.
		Subject:     user.PublicID,
		Role:        user.Role,
		Email:       user.Email,
		Phone:       user.Phone,
		Provider:    user.Provider,
		DisplayName: user.DisplayName,
	}

	access, err := u.issuer.AccessToken(claims, u.accessTTL)
	if err != nil {
		return nil, err
	}
	refresh, err := u.issuer.RefreshToken(claims, u.refreshTTL)
	if err != nil {
		return nil, err
	}

	return &SessionResponse{
		AccessToken:  access,
		RefreshToken: refresh,
		TokenType:    "Bearer",
		ExpiresIn:    int(u.accessTTL.Seconds()),
		User: &SessionUser{
			ID:          user.PublicID,
			DisplayName: user.DisplayName,
			Email:       user.Email,
			AvatarURL:   user.AvatarURL,
			Provider:    user.Provider,
			IsGuest:     isGuest,
		},
	}, nil
}

// applyIdentity copies the provider's current name and avatar onto the user,
// reporting whether anything actually changed so an unchanged sign-in does not
// cost a write.
func applyIdentity(user *domain.User, displayName, avatarURL string) bool {
	changed := false
	if displayName != "" && displayName != user.DisplayName {
		user.DisplayName = displayName
		changed = true
	}
	if avatarURL != "" && (user.AvatarURL == nil || *user.AvatarURL != avatarURL) {
		user.AvatarURL = &avatarURL
		changed = true
	}
	return changed
}
