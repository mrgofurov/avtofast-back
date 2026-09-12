package crypto

import (
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
)

// GenerateKeyPair generates a new Ed25519 key pair
func GenerateKeyPair() (ed25519.PublicKey, ed25519.PrivateKey, error) {
	return ed25519.GenerateKey(rand.Reader)
}

// SignEd25519 signs data with an Ed25519 private key and returns base64 string
func SignEd25519(privateKey ed25519.PrivateKey, message []byte) string {
	sig := ed25519.Sign(privateKey, message)
	return base64.StdEncoding.EncodeToString(sig)
}

// VerifyEd25519 verifies base64 signature against message and public key
func VerifyEd25519(publicKey ed25519.PublicKey, message []byte, base64Sig string) bool {
	sig, err := base64.StdEncoding.DecodeString(base64Sig)
	if err != nil {
		return false
	}
	return ed25519.Verify(publicKey, message, sig)
}

// SHA256Hex calculates SHA-256 hash returned as hex string
func SHA256Hex(data []byte) string {
	h := sha256.Sum256(data)
	return hex.EncodeToString(h[:])
}
