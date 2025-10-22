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

	hash := hmac.New(sha256.New, key)
	hash.Write(formMessage(sessionID, randValue))
	hmacValue := hash.Sum(nil)

	return hex.EncodeToString(hmacValue) + "." + hex.EncodeToString([]byte(randValue))
}

func Validate(token, sessionID string, key []byte) bool {
	parts := strings.Split(token, ".")
	if len(parts) != 2 {
		return false
	}

	receivedHmacValue, err := hex.DecodeString(parts[0])
	if err != nil {
		return false
	}

	randValue, err := hex.DecodeString(parts[1])
	if err != nil {
		return false
	}

	hash := hmac.New(sha256.New, key)
	hash.Write(formMessage(sessionID, string(randValue)))
	expectedHmacValue := hash.Sum(nil)

	return hmac.Equal(receivedHmacValue, expectedHmacValue)
}
