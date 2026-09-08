package ulid

import (
	"crypto/rand"
	"fmt"
	"io"
	"sync"
	"time"
)

const (
	Encoding = "0123456789ABCDEFGHJKMNPQRSTVWXYZ"
)

var (
	defaultMonotonic = &MonotonicGenerator{
		entropy: rand.Reader,
	}
	decTable [256]byte
)

func init() {
	for i := range decTable {
		decTable[i] = 0xFF
	}
	for i := 0; i < len(Encoding); i++ {
		decTable[Encoding[i]] = byte(i)
		// Crockford allows lowercase as well
		if Encoding[i] >= 'A' && Encoding[i] <= 'Z' {
			decTable[Encoding[i]+32] = byte(i)
		}
	}
}

// MonotonicGenerator produces strictly increasing ULIDs within the same millisecond.
type MonotonicGenerator struct {
	mu       sync.Mutex
	entropy  io.Reader
	lastTime uint64
	lastRand [10]byte
}

// New generates a new 26-character ULID using the current UTC time.
func New() string {
	return defaultMonotonic.NewAt(time.Now().UTC())
}

// NewAt generates a new 26-character ULID for the given time.
func NewAt(t time.Time) string {
	return defaultMonotonic.NewAt(t)
}

// NewAt generates a monotonic ULID for a specific time.
func (g *MonotonicGenerator) NewAt(t time.Time) string {
	ms := uint64(t.UnixMilli())

	g.mu.Lock()
	defer g.mu.Unlock()

	if ms == g.lastTime {
		// Increment entropy monotonically
		for i := len(g.lastRand) - 1; i >= 0; i-- {
			g.lastRand[i]++
			if g.lastRand[i] != 0 {
				break
			}
		}
	} else {
		g.lastTime = ms
		if _, err := io.ReadFull(g.entropy, g.lastRand[:]); err != nil {
			panic(fmt.Sprintf("ulid: failed to read entropy: %v", err))
		}
	}

	var raw [16]byte
	raw[0] = byte(ms >> 40)
	raw[1] = byte(ms >> 32)
	raw[2] = byte(ms >> 24)
	raw[3] = byte(ms >> 16)
	raw[4] = byte(ms >> 8)
	raw[5] = byte(ms)
	copy(raw[6:], g.lastRand[:])

	return encode(raw)
}

// IsValid checks if the provided string is a valid 26-character ULID.
func IsValid(s string) bool {
	if len(s) != 26 {
		return false
	}
	for i := 0; i < 26; i++ {
		if decTable[s[i]] == 0xFF {
			return false
		}
	}
	// The first character can only be 0-7 (max 48-bit timestamp)
	return decTable[s[0]] <= 7
}

func encode(raw [16]byte) string {
	var dst [26]byte

	// 10 chars timestamp (48 bits)
	dst[0] = Encoding[(raw[0]&224)>>5]
	dst[1] = Encoding[raw[0]&31]
	dst[2] = Encoding[(raw[1]&248)>>3]
	dst[3] = Encoding[((raw[1]&7)<<2)|((raw[2]&192)>>6)]
	dst[4] = Encoding[(raw[2]&62)>>1]
	dst[5] = Encoding[((raw[2]&1)<<4)|((raw[3]&240)>>4)]
	dst[6] = Encoding[((raw[3]&15)<<1)|((raw[4]&128)>>7)]
	dst[7] = Encoding[(raw[4]&124)>>2]
	dst[8] = Encoding[((raw[4]&3)<<3)|((raw[5]&224)>>5)]
	dst[9] = Encoding[raw[5]&31]

	// 16 chars entropy (80 bits)
	dst[10] = Encoding[(raw[6]&248)>>3]
	dst[11] = Encoding[((raw[6]&7)<<2)|((raw[7]&192)>>6)]
	dst[12] = Encoding[(raw[7]&62)>>1]
	dst[13] = Encoding[((raw[7]&1)<<4)|((raw[8]&240)>>4)]
	dst[14] = Encoding[((raw[8]&15)<<1)|((raw[9]&128)>>7)]
	dst[15] = Encoding[(raw[9]&124)>>2]
	dst[16] = Encoding[((raw[9]&3)<<3)|((raw[10]&224)>>5)]
	dst[17] = Encoding[raw[10]&31]
	dst[18] = Encoding[(raw[11]&248)>>3]
	dst[19] = Encoding[((raw[11]&7)<<2)|((raw[12]&192)>>6)]
	dst[20] = Encoding[(raw[12]&62)>>1]
	dst[21] = Encoding[((raw[12]&1)<<4)|((raw[13]&240)>>4)]
	dst[22] = Encoding[((raw[13]&15)<<1)|((raw[14]&128)>>7)]
	dst[23] = Encoding[(raw[14]&124)>>2]
	dst[24] = Encoding[((raw[14]&3)<<3)|((raw[15]&224)>>5)]
	dst[25] = Encoding[raw[15]&31]

	return string(dst[:])
}
