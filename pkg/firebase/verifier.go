// Package firebase verifies Firebase Authentication ID tokens.
//
// The mobile client signs in with Google or Apple through Firebase and ends up
// holding a Firebase ID token: an RS256 JWT signed by Google, not by us. The
// rest of the API only understands our own HS256 session tokens (see pkg/jwt),
// so `POST /v1/auth/session` exchanges one for the other and this package is
// the half that checks the incoming Firebase token is genuine.
package firebase

import (
	"context"
	"crypto/rsa"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

var (
	ErrDisabled       = errors.New("firebase verification is not configured")
	ErrTokenInvalid   = errors.New("firebase id token is invalid")
	ErrKeyUnavailable = errors.New("firebase signing keys are unavailable")
)

// publicKeysURL serves the x509 certificates Google signs ID tokens with. The
// keys rotate roughly daily; the response's Cache-Control max-age says when.
const publicKeysURL = "https://www.googleapis.com/robot/v1/metadata/x509/securetoken@system.gserviceaccount.com"

// Identity is what a verified ID token tells us about the signed-in learner.
type Identity struct {
	// Subject is the Firebase UID — stable for the life of the account, and
	// what the user row is keyed on.
	Subject     string
	Email       string
	Phone       string
	DisplayName string
	AvatarURL   string
	// SignInProvider is `google.com`, `apple.com`, `anonymous`, etc., taken
	// from the `firebase.sign_in_provider` claim.
	SignInProvider string
	EmailVerified  bool
}

// Verifier checks Firebase ID tokens against Google's rotating public keys.
//
// Safe for concurrent use: the key cache is behind a mutex and refreshed at
// most once per expiry window rather than per request.
type Verifier struct {
	projectID string
	client    *http.Client

	mu        sync.RWMutex
	keys      map[string]*rsa.PublicKey
	keysUntil time.Time
}

// NewVerifier returns a verifier for one Firebase project. An empty projectID
// yields a verifier whose Verify always fails with [ErrDisabled], which is how
// a deployment with no Firebase configured refuses sign-in cleanly instead of
// accepting anything.
func NewVerifier(projectID string) *Verifier {
	return &Verifier{
		projectID: projectID,
		client:    &http.Client{Timeout: 5 * time.Second},
		keys:      map[string]*rsa.PublicKey{},
	}
}

func (v *Verifier) Enabled() bool { return v.projectID != "" }

// Verify parses and validates a Firebase ID token, checking the signature
// against Google's current keys plus the issuer, audience and expiry that
// Firebase's own documented verification procedure requires.
func (v *Verifier) Verify(ctx context.Context, tokenStr string) (*Identity, error) {
	if !v.Enabled() {
		return nil, ErrDisabled
	}
	if tokenStr == "" {
		return nil, ErrTokenInvalid
	}

	token, err := jwt.Parse(
		tokenStr,
		func(token *jwt.Token) (any, error) {
			if _, ok := token.Method.(*jwt.SigningMethodRSA); !ok {
				return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
			}
			kid, _ := token.Header["kid"].(string)
			if kid == "" {
				return nil, ErrTokenInvalid
			}
			return v.keyFor(ctx, kid)
		},
		jwt.WithValidMethods([]string{"RS256"}),
		jwt.WithIssuer("https://securetoken.google.com/"+v.projectID),
		jwt.WithAudience(v.projectID),
	)
	if err != nil || !token.Valid {
		return nil, ErrTokenInvalid
	}

	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok {
		return nil, ErrTokenInvalid
	}

	// Firebase puts the real user id in `sub`; `user_id` carries the same
	// value and is only read as a fallback for older token formats.
	subject, _ := claims["sub"].(string)
	if subject == "" {
		subject, _ = claims["user_id"].(string)
	}
	if subject == "" {
		return nil, ErrTokenInvalid
	}

	identity := &Identity{
		Subject:        subject,
		SignInProvider: "firebase",
	}
	identity.Email, _ = claims["email"].(string)
	identity.Phone, _ = claims["phone_number"].(string)
	identity.DisplayName, _ = claims["name"].(string)
	identity.AvatarURL, _ = claims["picture"].(string)
	identity.EmailVerified, _ = claims["email_verified"].(bool)

	if fb, ok := claims["firebase"].(map[string]any); ok {
		if provider, ok := fb["sign_in_provider"].(string); ok && provider != "" {
			identity.SignInProvider = provider
		}
	}

	return identity, nil
}

func (v *Verifier) keyFor(ctx context.Context, kid string) (*rsa.PublicKey, error) {
	v.mu.RLock()
	key, ok := v.keys[kid]
	fresh := time.Now().Before(v.keysUntil)
	v.mu.RUnlock()
	if ok && fresh {
		return key, nil
	}

	// Either the cache expired or the token was signed with a key we have not
	// seen — both mean refetching, because Google rotates keys without notice.
	if err := v.refreshKeys(ctx); err != nil {
		// A rotation we failed to fetch should not invalidate a token we can
		// still verify with a key already in hand.
		if ok {
			return key, nil
		}
		return nil, err
	}

	v.mu.RLock()
	key, ok = v.keys[kid]
	v.mu.RUnlock()
	if !ok {
		return nil, ErrTokenInvalid
	}
	return key, nil
}

func (v *Verifier) refreshKeys(ctx context.Context) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, publicKeysURL, nil)
	if err != nil {
		return ErrKeyUnavailable
	}
	res, err := v.client.Do(req)
	if err != nil {
		return ErrKeyUnavailable
	}
	defer func() { _ = res.Body.Close() }()
	if res.StatusCode != http.StatusOK {
		return ErrKeyUnavailable
	}

	var certs map[string]string
	if err := json.NewDecoder(res.Body).Decode(&certs); err != nil {
		return ErrKeyUnavailable
	}

	keys := make(map[string]*rsa.PublicKey, len(certs))
	for kid, pem := range certs {
		key, err := jwt.ParseRSAPublicKeyFromPEM([]byte(pem))
		if err != nil {
			continue
		}
		keys[kid] = key
	}
	if len(keys) == 0 {
		return ErrKeyUnavailable
	}

	v.mu.Lock()
	v.keys = keys
	// Google's own clients honour the response's max-age; an hour is well
	// inside the ~24h rotation window and costs one request per hour.
	v.keysUntil = time.Now().Add(time.Hour)
	v.mu.Unlock()
	return nil
}
