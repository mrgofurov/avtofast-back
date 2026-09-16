package jwt

import (
	"errors"
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

var (
	ErrTokenExpired = errors.New("token has expired")
	ErrTokenInvalid = errors.New("token is invalid")
	ErrNoSubject    = errors.New("token subject is missing")
)

// TokenClaims represents standard and custom claims from Firebase/Supabase/Internal JWT
type TokenClaims struct {
	Subject   string   `json:"sub"`
	Issuer    string   `json:"iss"`
	Audience  []string `json:"aud"`
	ExpiresAt int64    `json:"exp"`
	Role      string   `json:"role"`
	Email     string   `json:"email,omitempty"`
	Phone     string   `json:"phone_number,omitempty"`
	Provider  string   `json:"firebase_sign_in_provider,omitempty"`

	// TokenType separates an access token from a refresh token. A refresh
	// token is only ever accepted by POST /v1/auth/refresh; letting one
	// through as a bearer credential would hand out an access lifetime
	// measured in months.
	TokenType string `json:"typ,omitempty"`
}

// Token types carried in the `typ` claim.
const (
	TokenTypeAccess  = "access"
	TokenTypeRefresh = "refresh"
)

// ErrWrongTokenType is returned when a refresh token is presented where an
// access token is required, or vice versa.
var ErrWrongTokenType = errors.New("token has the wrong type")

// TokenVerifier defines the interface for token verification
type TokenVerifier interface {
	Verify(tokenString string) (*TokenClaims, error)
}

// VerifierConfig holds options for JWT verification
type VerifierConfig struct {
	SecretKey        string
	ExpectedIssuer   string
	ExpectedAudience string
	SkipExpiryCheck  bool
}

// Verifier implements TokenVerifier
type Verifier struct {
	config VerifierConfig
}

// NewVerifier creates a new JWT Verifier
func NewVerifier(config VerifierConfig) *Verifier {
	return &Verifier{config: config}
}

// Verify parses and verifies a JWT token
func (v *Verifier) Verify(tokenStr string) (*TokenClaims, error) {
	if tokenStr == "" {
		return nil, ErrTokenInvalid
	}

	token, err := jwt.Parse(tokenStr, func(token *jwt.Token) (any, error) {
		// Verify signing method
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); ok {
			return []byte(v.config.SecretKey), nil
		}
		return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
	})

	if err != nil {
		if errors.Is(err, jwt.ErrTokenExpired) {
			return nil, ErrTokenExpired
		}
		return nil, ErrTokenInvalid
	}

	if !token.Valid {
		return nil, ErrTokenInvalid
	}

	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok {
		return nil, ErrTokenInvalid
	}

	sub, _ := claims["sub"].(string)
	if sub == "" {
		return nil, ErrNoSubject
	}

	role, _ := claims["role"].(string)
	if role == "" {
		// check nested user_metadata or app_metadata for Supabase
		if appMeta, ok := claims["app_metadata"].(map[string]any); ok {
			if r, ok := appMeta["role"].(string); ok {
				role = r
			}
		}
	}
	if role == "" {
		role = "user"
	}

	email, _ := claims["email"].(string)
	phone, _ := claims["phone"].(string)
	iss, _ := claims["iss"].(string)

	prov, _ := claims["provider"].(string)
	if prov == "" {
		prov = "firebase"
	}

	typ, _ := claims["typ"].(string)
	if typ == "" {
		typ = TokenTypeAccess
	}
	if typ != TokenTypeAccess {
		return nil, ErrWrongTokenType
	}

	return &TokenClaims{
		Subject:   sub,
		Issuer:    iss,
		Role:      role,
		Email:     email,
		Phone:     phone,
		Provider:  prov,
		TokenType: typ,
	}, nil
}

// VerifyRefresh parses a refresh token, applying the same signature and expiry
// checks as [Verifier.Verify] but requiring `typ: refresh`.
func (v *Verifier) VerifyRefresh(tokenStr string) (*TokenClaims, error) {
	claims, err := v.parse(tokenStr)
	if err != nil {
		return nil, err
	}
	if typ, _ := claims["typ"].(string); typ != TokenTypeRefresh {
		return nil, ErrWrongTokenType
	}
	sub, _ := claims["sub"].(string)
	if sub == "" {
		return nil, ErrNoSubject
	}
	role, _ := claims["role"].(string)
	if role == "" {
		role = "user"
	}
	email, _ := claims["email"].(string)
	prov, _ := claims["provider"].(string)
	if prov == "" {
		prov = "firebase"
	}
	iss, _ := claims["iss"].(string)
	return &TokenClaims{
		Subject:   sub,
		Issuer:    iss,
		Role:      role,
		Email:     email,
		Provider:  prov,
		TokenType: TokenTypeRefresh,
	}, nil
}

// parse does the signature and expiry half of verification, shared by the
// access and refresh paths.
func (v *Verifier) parse(tokenStr string) (jwt.MapClaims, error) {
	if tokenStr == "" {
		return nil, ErrTokenInvalid
	}
	token, err := jwt.Parse(tokenStr, func(token *jwt.Token) (any, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); ok {
			return []byte(v.config.SecretKey), nil
		}
		return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
	})
	if err != nil {
		if errors.Is(err, jwt.ErrTokenExpired) {
			return nil, ErrTokenExpired
		}
		return nil, ErrTokenInvalid
	}
	if !token.Valid {
		return nil, ErrTokenInvalid
	}
	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok {
		return nil, ErrTokenInvalid
	}
	return claims, nil
}

// SessionClaims is everything a minted session token carries about its owner.
type SessionClaims struct {
	Subject     string
	Role        string
	Email       string
	Phone       string
	Provider    string
	DisplayName string
}

// Issuer mints the API's own HS256 session tokens.
//
// Nothing else in the system issues credentials: a Firebase ID token, a
// device-scoped guest identity and a refresh token all funnel through here, so
// there is exactly one place that decides what a session may claim.
type Issuer struct {
	secret   string
	issuer   string
	audience string
}

func NewIssuer(secret, issuer, audience string) *Issuer {
	return &Issuer{secret: secret, issuer: issuer, audience: audience}
}

func (i *Issuer) sign(c SessionClaims, tokenType string, ttl time.Duration) (string, error) {
	now := time.Now().UTC()
	claims := jwt.MapClaims{
		"sub":      c.Subject,
		"role":     c.Role,
		"iss":      i.issuer,
		"aud":      i.audience,
		"provider": c.Provider,
		"typ":      tokenType,
		"iat":      now.Unix(),
		"exp":      now.Add(ttl).Unix(),
	}
	if c.Email != "" {
		claims["email"] = c.Email
	}
	if c.Phone != "" {
		claims["phone"] = c.Phone
	}
	if c.DisplayName != "" {
		claims["name"] = c.DisplayName
	}
	return jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString([]byte(i.secret))
}

// AccessToken mints the bearer token every authenticated endpoint accepts.
func (i *Issuer) AccessToken(c SessionClaims, ttl time.Duration) (string, error) {
	return i.sign(c, TokenTypeAccess, ttl)
}

// RefreshToken mints the long-lived token POST /v1/auth/refresh trades for a
// fresh access token, so a learner is not signed out every day.
func (i *Issuer) RefreshToken(c SessionClaims, ttl time.Duration) (string, error) {
	return i.sign(c, TokenTypeRefresh, ttl)
}

// GenerateTestToken generates a signed JWT token for testing and seeding
func GenerateTestToken(sub, role, secret string, duration time.Duration, email ...string) (string, error) {
	now := time.Now().UTC()
	userEmail := ""
	if len(email) > 0 {
		userEmail = email[0]
	}
	claims := jwt.MapClaims{
		"sub":      sub,
		"role":     role,
		"iss":      "https://api.avtofast.uz/auth",
		"aud":      "avtofast-api",
		"provider": "firebase",
		"typ":      TokenTypeAccess,
		"email":    userEmail,
		"iat":      now.Unix(),
		"exp":      now.Add(duration).Unix(),
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString([]byte(secret))
}
