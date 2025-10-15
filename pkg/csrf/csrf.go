package csrf

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strings"
)

const keyLength = 64

func formMessage(sessionID, randValue string) []byte {
	return fmt.Appendf(nil, "%d!%s!%d!%s", len(sessionID), sessionID, len(randValue), randValue)
}

func NewToken(sessionID string, key []byte) string {
	buf := make([]byte, keyLength)
	_, _ = rand.Read(buf)

	randValue := hex.EncodeToString(buf)

	msg := formMessage(sessionID, randValue)
	hash := hmac.New(sha256.New, key)
	return hex.EncodeToString(hash.Sum(msg)) + "." + hex.EncodeToString([]byte(randValue))
}

func Validate(token, sessionID string, key []byte) bool {
	parts := strings.Split(token, ".")
	if len(parts) != 2 {
		return false
	}

	mac, err := hex.DecodeString(parts[0])
	if err != nil {
		return false
	}

	randValue, err := hex.DecodeString(parts[1])
	if err != nil {
		return false
	}

	msg := formMessage(sessionID, string(randValue))
	expectedHash := hmac.New(sha256.New, key)

	return hmac.Equal(mac, expectedHash.Sum(msg))
}
