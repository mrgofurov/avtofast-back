package jwt_test

import (
	"testing"
	"time"

	"github.com/avtofast/avtofast-back/pkg/jwt"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestJWTVerification(t *testing.T) {
	secret := "my-very-secret-test-key-32-chars-long"
	verifier := jwt.NewVerifier(jwt.VerifierConfig{
		SecretKey:        secret,
		ExpectedIssuer:   "https://api.avtotest.uz/auth",
		ExpectedAudience: "avtofast-api",
	})

	tokenStr, err := jwt.GenerateTestToken("usr_test_123", "user", secret, 1*time.Hour)
	require.NoError(t, err)
	assert.NotEmpty(t, tokenStr)

	claims, err := verifier.Verify(tokenStr)
	require.NoError(t, err)
	assert.Equal(t, "usr_test_123", claims.Subject)
	assert.Equal(t, "user", claims.Role)

	// Expired token test
	expiredToken, err := jwt.GenerateTestToken("usr_expired", "user", secret, -1*time.Hour)
	require.NoError(t, err)
	_, err = verifier.Verify(expiredToken)
	assert.Error(t, err)

	// Invalid signature test
	_, err = verifier.Verify(tokenStr + "tampered")
	assert.Error(t, err)
}
