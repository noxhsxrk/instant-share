package token

import (
	"crypto/rand"
	"encoding/hex"
)

func Generate() string {
	bytes := make([]byte, 4)
	rand.Read(bytes)
	return hex.EncodeToString(bytes)
}
