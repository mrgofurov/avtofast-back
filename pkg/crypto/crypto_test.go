package crypto_test

import (
	"testing"

	"github.com/avtofast/avtofast-back/pkg/crypto"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestEd25519SigningAndVerification(t *testing.T) {
	pub, priv, err := crypto.GenerateKeyPair()
	require.NoError(t, err)

	message := []byte("uz-theory-2026-09:2026.09.1:manifest-sha256")
	sig := crypto.SignEd25519(priv, message)
	assert.NotEmpty(t, sig)

	// Verify valid signature
	valid := crypto.VerifyEd25519(pub, message, sig)
	assert.True(t, valid)

	// Verify tampered message fails
	tampered := []byte("uz-theory-2026-09:2026.09.2:manifest-sha256")
	assert.False(t, crypto.VerifyEd25519(pub, tampered, sig))

	// Verify tampered signature fails
	assert.False(t, crypto.VerifyEd25519(pub, message, "invalid-base64"))
}

func TestSHA256Hex(t *testing.T) {
	data := []byte("hello avtofast")
	hash := crypto.SHA256Hex(data)
	assert.Len(t, hash, 64)
}
