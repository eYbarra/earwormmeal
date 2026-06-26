package identity

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/binary"
	"strconv"
	"time"
)

// Generator produces deterministic display names from IP addresses
// using HMAC-SHA256 with a secret salt and epoch-based rotation.
type Generator struct {
	salt         []byte
	rotationSecs int64
}

// New creates a Generator with the given salt and rotation interval.
// Panics if salt is zero-length.
func New(salt []byte, rotationHours int) *Generator {
	if len(salt) == 0 {
		panic("identity: salt must not be empty")
	}
	return &Generator{
		salt:         salt,
		rotationSecs: int64(rotationHours) * 3600,
	}
}

// Generate produces a deterministic "Adjective Noun" display name for the given IP.
func (g *Generator) Generate(ip string) string {
	epoch := g.currentEpoch()
	effSalt := g.effectiveSalt(epoch)
	mac := computeHMAC(effSalt, ip)

	adjIndex := binary.BigEndian.Uint32(mac[0:4]) % uint32(len(adjectives))
	nounIndex := binary.BigEndian.Uint32(mac[4:8]) % uint32(len(nouns))

	return adjectives[adjIndex] + " " + nouns[nounIndex]
}

// currentEpoch computes the current rotation epoch.
func (g *Generator) currentEpoch() int64 {
	return time.Now().Unix() / g.rotationSecs
}

// effectiveSalt derives the epoch-specific salt by appending ":epoch" to the base salt.
func (g *Generator) effectiveSalt(epoch int64) []byte {
	suffix := ":" + strconv.FormatInt(epoch, 10)
	result := make([]byte, len(g.salt)+len(suffix))
	copy(result, g.salt)
	copy(result[len(g.salt):], suffix)
	return result
}

// computeHMAC computes HMAC-SHA256 with the given key and message.
func computeHMAC(key []byte, message string) [32]byte {
	h := hmac.New(sha256.New, key)
	h.Write([]byte(message))
	sum := h.Sum(nil)
	var result [32]byte
	copy(result[:], sum)
	return result
}
