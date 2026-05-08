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

// Engine synthesizes gesture segments using continuous motion
type Engine struct {
	KeySignature uint64
	SegmentLen   int
	State        *State
}

// NewEngine creates a new motion synthesis engine
func NewEngine(keyData []float64, segmentLen int) *Engine {
	return &Engine{
		KeySignature: crypto.KeySignature(keyData),
		SegmentLen:   segmentLen,
		State:        &State{Position: 0, Velocity: 0, LastGate: 1.0},
	}
}

// SynthesizeSegment generates audio for one byte using seeded gesture policy
func (e *Engine) SynthesizeSegment(source []float64, byteVal byte, position int64) []float64 {
	seed := crypto.SeedGen(e.KeySignature, byteVal, position)
	intensity := 0.35 + 0.65*float64(byteVal&0x0f)/15.0
	policy := gesture.ComputeGesturePolicy(seed, byteVal, intensity)

	segment := make([]float64, e.SegmentLen)

	for i := 0; i < e.SegmentLen; i++ {
		t := float64(i) / float64(e.SegmentLen-1)

		targetVel := policy.VelocityCurve(t, policy.Intensity)
		e.State.Velocity += clamp(targetVel-e.State.Velocity, -0.08, 0.08)

		e.State.Position = audio.WrapPosition(e.State.Position+e.State.Velocity, len(source))

		sample := audio.SampleAt(source, e.State.Position)

		gate := policy.GatingCurve(t, policy.Intensity)
		smoothedGate := smoothGate(gate, t)

		segment[i] = sample * smoothedGate
		e.State.LastGate = gate
	}

	return segment
}

// Reset clears the motion state
func (e *Engine) Reset() {
	e.State = &State{Position: 0, Velocity: 0, LastGate: 1.0}
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
