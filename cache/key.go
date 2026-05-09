package cache

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strings"
)

const (
	keySeparator     = ":"
	hashKeySeparator = "|"
)

func BuildKey(parts ...any) string {
	if len(parts) == 0 {
		return ""
	}

	return joinParts(parts, keySeparator)
}

func BuildHashedKey(parts ...any) string {
	sum := sha256.Sum256([]byte(joinParts(parts, hashKeySeparator)))

	return hex.EncodeToString(sum[:])
}

func joinParts(parts []any, separator string) string {
	stringParts := make([]string, 0, len(parts))
	for _, part := range parts {
		stringParts = append(stringParts, fmt.Sprint(part))
	}

	return strings.Join(stringParts, separator)
}
