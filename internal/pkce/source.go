package pkce

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"math/big"
)

const MethodS256 = "S256"

type Source struct{}

func (p Source) randBytes(n int) []byte {
	b := make([]byte, n)
	_, _ = rand.Read(b)

	return b
}

func (p Source) randString(n int) string {
	const letters = "0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz-"

	ret := make([]byte, n)
	for i := range n {
		num, _ := rand.Int(rand.Reader, big.NewInt(int64(len(letters))))
		ret[i] = letters[num.Int64()]
	}

	return string(ret)
}

func (p Source) PKCE() PKCE {
	const n = 32

	verifierBuf := make([]byte, base64.RawURLEncoding.EncodedLen(n))
	base64.RawURLEncoding.Encode(verifierBuf, p.randBytes(n))

	challengeSHA := sha256.Sum256(verifierBuf)
	challengeBuf := make([]byte, base64.RawURLEncoding.EncodedLen(len(challengeSHA)))
	base64.RawURLEncoding.Encode(challengeBuf, challengeSHA[:])

	return PKCE{
		Verifier:  string(verifierBuf),
		Challenge: string(challengeBuf),
		Method:    MethodS256,
	}
}

func (p Source) State() string {
	return p.randString(64)
}

func (p Source) SessionID() string {
	return p.randString(32) // Entropy E = L * log2(63) = 32 * log2(63) = 191.3 bits
}
