package runtime

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"time"
)

type CryptoIDGenerator struct{}

func (CryptoIDGenerator) New() (string, error) {
	var value [16]byte
	if _, err := rand.Read(value[:]); err != nil {
		return "", fmt.Errorf("read secure randomness: %w", err)
	}
	return hex.EncodeToString(value[:]), nil
}

type UTCClock struct{}

func (UTCClock) Now() time.Time { return time.Now().UTC() }
