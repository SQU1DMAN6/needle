package crypto

import (
	"crypto/sha256"
	"encoding/binary"
	"math"
)

// SeedGen generates a deterministic seed from key signature, byte value, and position
// Enhanced with state mixing for better security
func SeedGen(keySignature uint64, byteValue byte, position int64) uint64 {
	h := sha256.New()
	buf := make([]byte, 24)
	binary.BigEndian.PutUint64(buf[0:8], keySignature)
	binary.BigEndian.PutUint64(buf[8:16], uint64(position))
	// Add position MSB to create temporal encoding
	binary.BigEndian.PutUint64(buf[16:24], uint64(position>>32)^keySignature)
	h.Write(buf)
	h.Write([]byte{byteValue})
	hash := h.Sum(nil)
	return binary.BigEndian.Uint64(hash[0:8])
}

// SeedGenWithContext generates a seed that includes performance context
// for state-dependent gesture synthesis
func SeedGenWithContext(keySignature uint64, byteValue byte, position int64,
	phraseBeat float64, energy float64, lastGesture int) uint64 {
	h := sha256.New()
	buf := make([]byte, 40)
	binary.BigEndian.PutUint64(buf[0:8], keySignature)
	binary.BigEndian.PutUint64(buf[8:16], uint64(position))
	binary.BigEndian.PutUint64(buf[16:24], math.Float64bits(phraseBeat))
	binary.BigEndian.PutUint64(buf[24:32], math.Float64bits(energy))
	binary.BigEndian.PutUint32(buf[32:36], uint32(lastGesture))
	// Mix byte value across multiple positions
	buf[36] = byteValue
	buf[37] = byte(byteValue ^ uint8(position))
	buf[38] = byte(byteValue ^ uint8(phraseBeat*255))
	buf[39] = byte(byteValue ^ uint8(energy*255))
	h.Write(buf)
	hash := h.Sum(nil)
	return binary.BigEndian.Uint64(hash[0:8])
}

// Seeded PRNG using xorshift64*
type PRNG struct {
	state uint64
}

// NewPRNG creates a new PRNG with a given seed
func NewPRNG(seed uint64) *PRNG {
	if seed == 0 {
		seed = 1
	}
	return &PRNG{state: seed}
}

// Next returns the next random float64 in [0, 1)
func (p *PRNG) Next() float64 {
	p.state ^= p.state << 13
	p.state ^= p.state >> 7
	p.state ^= p.state << 17
	return float64(p.state) / float64(^uint64(0))
}

// NextInt returns a random int in [0, max)
func (p *PRNG) NextInt(max int) int {
	return int(p.Next() * float64(max))
}

// NextFloat returns a random float64 in [min, max)
func (p *PRNG) NextFloat(min, max float64) float64 {
	return min + p.Next()*(max-min)
}

// KeySignature computes a signature from audio data for key-conditioned behaviour
func KeySignature(data []float64) uint64 {
	h := sha256.New()
	if len(data) > 10000 {
		for _, v := range data[:10000] {
			bits := math.Float64bits(v)
			buf := make([]byte, 8)
			binary.BigEndian.PutUint64(buf, bits)
			h.Write(buf)
		}
	} else {
		for _, v := range data {
			bits := math.Float64bits(v)
			buf := make([]byte, 8)
			binary.BigEndian.PutUint64(buf, bits)
			h.Write(buf)
		}
	}
	hashBytes := h.Sum(nil)
	return binary.BigEndian.Uint64(hashBytes[0:8])
}
