package testutil

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"math/big"
	"os"
	"testing"
	"time"
)

const (
	defaultEmailRandomLength = 10
	uuidByteLength           = 16
	uuidVersion4Bits         = 0x40
	uuidVariantBits          = 0x80
	uuidVersion4Mask         = 0x0f
	uuidVariantMask          = 0x3f
)

func RandomString(length int) string {
	bytes := make([]byte, length/2+1)
	if _, err := rand.Read(bytes); err != nil {
		return ""
	}

	return hex.EncodeToString(bytes)[:length]
}

func RandomInt(minVal, maxVal int64) int64 {
	if minVal >= maxVal {
		return minVal
	}

	n, err := rand.Int(rand.Reader, big.NewInt(maxVal-minVal+1))
	if err != nil {
		return minVal
	}

	return n.Int64() + minVal
}

func RandomEmail() string {
	return fmt.Sprintf("test_%s@example.com", RandomString(defaultEmailRandomLength))
}

func RandomUUID() string {
	bytes := make([]byte, uuidByteLength)
	if _, err := rand.Read(bytes); err != nil {
		return ""
	}
	// Set version (4) and variant bits
	bytes[6] = (bytes[6] & uuidVersion4Mask) | uuidVersion4Bits
	bytes[8] = (bytes[8] & uuidVariantMask) | uuidVariantBits

	return fmt.Sprintf("%x-%x-%x-%x-%x",
		bytes[0:4], bytes[4:6], bytes[6:8], bytes[8:10], bytes[10:16])
}

func Eventually(t *testing.T, condition func() bool, timeout time.Duration, interval time.Duration) {
	t.Helper()

	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if condition() {
			return
		}

		time.Sleep(interval)
	}

	t.Fatal("Condition not met within timeout")
}

func EventuallyWithMessage(t *testing.T, condition func() bool, timeout time.Duration, interval time.Duration, message string) {
	t.Helper()

	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if condition() {
			return
		}

		time.Sleep(interval)
	}

	t.Fatalf("Condition not met within timeout: %s", message)
}

func SkipIfShort(t *testing.T) {
	t.Helper()

	if testing.Short() {
		t.Skip("Skipping test in short mode")
	}
}

func RequireEnv(t *testing.T, key string) string {
	t.Helper()

	value := os.Getenv(key)

	if value == "" {
		t.Skipf("Environment variable %s is required but not set", key)
	}

	return value
}

func Parallel(t *testing.T) {
	t.Helper()
	t.Parallel()
}

func CleanupFunc(t *testing.T, fn func()) {
	t.Helper()
	t.Cleanup(fn)
}
