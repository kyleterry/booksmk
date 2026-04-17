package jobrunner

import (
	"crypto/rand"
	"math/big"
	"time"
)

// Jitter returns a duration between -offset and +offset.
func Jitter(offset time.Duration) time.Duration {
	if offset <= 0 {
		return 0
	}
	max := offset.Nanoseconds() * 2
	n, err := rand.Int(rand.Reader, big.NewInt(max))
	if err != nil {
		// fallback to no jitter on error
		return 0
	}
	return time.Duration(n.Uint64()) - offset
}
