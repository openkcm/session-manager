package session

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"io"
)

const csrfNonceSize = 16 // 128-bit nonce

// generateCSRFToken returns nonce.signature (both base64url, no padding)
func generateCSRFToken(sessionID string, secret []byte) (string, error) {
	if len(secret) < 32 {
		return "", errors.New("csrf secret too short (need >=32 bytes)")
	}
	nonce := make([]byte, csrfNonceSize)
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", err
	}
	mac := hmac.New(sha256.New, secret)
	mac.Write([]byte(sessionID))
	mac.Write(nonce)
	sig := mac.Sum(nil)

	return base64.RawURLEncoding.EncodeToString(nonce) + "." +
		base64.RawURLEncoding.EncodeToString(sig), nil
}

func validateCSRFToken(token, sessionID string, secret []byte) bool {
	dot := -1
	for i := range len(token) {
		if token[i] == '.' {
			dot = i
			break
		}
	}
	if dot <= 0 || dot == len(token)-1 {
		return false
	}
	nonceB64 := token[:dot]
	sigB64 := token[dot+1:]

	nonce, err1 := base64.RawURLEncoding.DecodeString(nonceB64)
	sig, err2 := base64.RawURLEncoding.DecodeString(sigB64)
	if err1 != nil || err2 != nil {
		return false
	}

	mac := hmac.New(sha256.New, secret)
	mac.Write([]byte(sessionID))
	mac.Write(nonce)
	expect := mac.Sum(nil)
	return hmac.Equal(sig, expect)
}
