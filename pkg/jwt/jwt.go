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
}

// TokenVerifier defines the interface for token verification
type TokenVerifier interface {
	Verify(tokenString string) (*TokenClaims, error)
}

// VerifierConfig holds options for JWT verification
type VerifierConfig struct {
	SecretKey       string
	ExpectedIssuer  string
	ExpectedAudience string
	SkipExpiryCheck bool
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

	return &TokenClaims{
		Subject:  sub,
		Issuer:   iss,
		Role:     role,
		Email:    email,
		Phone:    phone,
		Provider: prov,
	}, nil
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
		"iss":      "https://api.avtotest.uz/auth",
		"aud":      "avtofast-api",
		"provider": "firebase",
		"email":    userEmail,
		"iat":      now.Unix(),
		"exp":      now.Add(duration).Unix(),
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString([]byte(secret))
}
