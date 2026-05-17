package motion

import (
	"needle/internal/audio"
	"needle/internal/crypto"
	"needle/internal/gesture"
)

// State represents the continuous motion state
type State struct {
	Position float64
	Velocity float64
	LastGate float64
}

// Engine synthesizes gesture segments using motion
type Engine struct {
	KeySignature uint64
	SegmentLen   int
	State        *State
	SegmentIndex int64 // Track segment number for consistent seeding
}

// NewEngine creates a new motion synthesis engine
func NewEngine(keyData []float64, segmentLen int) *Engine {
	return &Engine{
		KeySignature: crypto.KeySignature(keyData),
		SegmentLen:   segmentLen,
		State:        &State{Position: 0, Velocity: 0, LastGate: 1.0},
		SegmentIndex: 0,
	}
}

// EventLen returns a variable event duration based on key, nibble, and engine position.
func (e *Engine) EventLen(byteVal byte) int {
	baseLen := int(0.22 * float64(audio.SampleRate))
	variance := int(0.08 * float64(audio.SampleRate))

	seed := crypto.SeedGen(e.KeySignature, byteVal, e.SegmentIndex)
	rand := crypto.NewPRNG(seed).Next()
	adjust := int((rand - 0.5) * 2.0 * float64(variance))
	// nibble-dependent timing bias adds key-conditioned grammar
	bias := int((float64(byteVal&0x0f)/15.0 - 0.5) * float64(variance/3))

	length := baseLen + adjust + bias
	minLen := int(0.16 * float64(audio.SampleRate))
	maxLen := int(0.30 * float64(audio.SampleRate))
	if length < minLen {
		length = minLen
	}
	if length > maxLen {
		length = maxLen
	}

	return length
}

// Clone duplicates engine state for stateful decode candidate expansion.
func (e *Engine) Clone() *Engine {
	stateCopy := *e.State
	return &Engine{
		KeySignature: e.KeySignature,
		SegmentLen:   e.SegmentLen,
		State:        &stateCopy,
		SegmentIndex: e.SegmentIndex,
	}
}

// SynthesizeEvent generates audio for a variable-length event using seeded gesture policy.
func (e *Engine) SynthesizeEvent(source []float64, byteVal byte) []float64 {
	eventLen := e.EventLen(byteVal)
	seed := crypto.SeedGen(e.KeySignature, byteVal, e.SegmentIndex)
	intensity := 0.35 + 0.65*float64(byteVal&0x0f)/15.0
	policy := gesture.ComputeGesturePolicy(seed, byteVal, intensity)

	segment := make([]float64, eventLen)

	for i := 0; i < eventLen; i++ {
		t := 0.0
		if eventLen > 1 {
			t = float64(i) / float64(eventLen-1)
		}

		targetVel := policy.VelocityCurve(t, policy.Intensity)
		e.State.Velocity += clamp(targetVel-e.State.Velocity, -0.08, 0.08)

		e.State.Position = audio.WrapPosition(e.State.Position+e.State.Velocity, len(source))

		sample := audio.SampleAt(source, e.State.Position)

		gate := policy.GatingCurve(t, policy.Intensity)
		smoothedGate := smoothGate(gate, t)

		segment[i] = sample * smoothedGate
		e.State.LastGate = gate
	}

	e.SegmentIndex++
	return segment
}

// SynthesizeSegment is retained for compatibility with older callers.
func (e *Engine) SynthesizeSegment(source []float64, byteVal byte, position int64) []float64 {
	return e.SynthesizeEvent(source, byteVal)
}

// Reset clears the motion state
func (e *Engine) Reset() {
	e.State = &State{Position: 0, Velocity: 0, LastGate: 1.0}
	e.SegmentIndex = 0
}

// ResetKeepingContext resets state but preserves physics memory
func (e *Engine) ResetKeepingContext() {
	e.State = &State{Position: 0, Velocity: 0, LastGate: 1.0}
	e.SegmentIndex = 0
}

func clamp(value, min, max float64) float64 {
	if value < min {
		return min
	}
	if value > max {
		return max
	}
	return value
}

func smoothGate(gate, t float64) float64 {
	if gate < 0.5 {
		return 0.0
	}
	attack := 0.01
	release := 0.01
	if t < attack {
		return gate * (t / attack)
	}
	if t > 1.0-release {
		return gate * ((1.0 - t) / release)
	}
	return gate
}
