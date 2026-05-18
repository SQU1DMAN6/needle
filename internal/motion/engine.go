package motion

import (
	"math"

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
	KeySignature  uint64
	SegmentLen    int
	State         *State
	SegmentIndex  int64 // Track segment number for consistent seeding
	Tempo         float64
	BeatPosition  float64
	PhraseEnergy  float64
	CrossfaderPos float64
	LastGesture   int
	LastIntensity float64
	Physics       *PhysicsState
	SegmentBuf    []float64
}

// NewEngine creates a new motion synthesis engine
func NewEngine(keyData []float64, segmentLen int) *Engine {
	keySig := crypto.KeySignature(keyData)
	tempo := 88.0 + float64((keySig>>32)%32) // 88-119 BPM for groove variety
	return &Engine{
		KeySignature:  keySig,
		SegmentLen:    segmentLen,
		State:         &State{Position: 0, Velocity: 0, LastGate: 1.0},
		SegmentIndex:  0,
		Tempo:         tempo,
		BeatPosition:  0.0,
		PhraseEnergy:  0.5,
		CrossfaderPos: 0.5,
		LastGesture:   -1,
		LastIntensity: 0.6,
		Physics:       NewPhysicsState(),
		SegmentBuf:    nil,
	}
}

// EventLen returns a variable event duration based on key, nibble, tempo, and groove.
// Realistic scratching events typically last 300-700ms for musical coherence.
func (e *Engine) EventLen(byteVal byte) int {
	beatSamples := float64(audio.SampleRate) * 60.0 / e.Tempo
	// Longer subdivisions for more realistic scratching:
	// 1 beat = ~680ms at 88 BPM, so these produce 300-850ms gestures
	subdivisions := []float64{0.5, 0.75, 1.0, 1.25, 1.5}

	seed := crypto.SeedGen(e.KeySignature, byteVal, e.SegmentIndex)
	rng := crypto.NewPRNG(seed)
	sub := subdivisions[rng.NextInt(len(subdivisions))]
	drift := (rng.Next() - 0.5) * 0.12 * beatSamples

	// Phrase-aware timing: favor longer endings at phrase boundaries
	phraseFactor := 1.0
	if e.BeatPosition > 3.0 {
		phraseFactor = 1.15
	}

	// Adjusted for faster events: clamp to 150-400ms
	length := int(sub*beatSamples*phraseFactor + drift)
	minLen := int(math.Round(0.15 * float64(audio.SampleRate))) // 150ms minimum
	maxLen := int(math.Round(0.40 * float64(audio.SampleRate))) // 400ms maximum
	if length < minLen {
		length = minLen
	}
	if length > maxLen {
		length = maxLen
	}

	return length
}

func (e *Engine) GetPerformanceContext() gesture.PerformanceContext {
	return gesture.PerformanceContext{
		PreviousType: e.LastGesture,
		Momentum:     e.Physics.GetMomentumInfluence(),
		Energy:       e.PhraseEnergy,
		BeatPhase:    math.Mod(e.BeatPosition, 1.0),
		Crossfader:   e.CrossfaderPos,
	}
}

// Clone duplicates engine state for stateful decode candidate expansion.
func (e *Engine) Clone() *Engine {
	stateCopy := *e.State
	var physicsCopy *PhysicsState
	if e.Physics != nil {
		p := *e.Physics
		physicsCopy = &p
	}
	return &Engine{
		KeySignature:  e.KeySignature,
		SegmentLen:    e.SegmentLen,
		State:         &stateCopy,
		SegmentIndex:  e.SegmentIndex,
		Tempo:         e.Tempo,
		BeatPosition:  e.BeatPosition,
		PhraseEnergy:  e.PhraseEnergy,
		CrossfaderPos: e.CrossfaderPos,
		LastGesture:   e.LastGesture,
		LastIntensity: e.LastIntensity,
		Physics:       physicsCopy,
		SegmentBuf:    nil,
	}
}

// SynthesizeEvent generates audio for a variable-length event using seeded gesture policy.
func (e *Engine) SynthesizeEvent(source []float64, byteVal byte) []float64 {
	eventLen := e.EventLen(byteVal)
	// Use context-aware seed generation for better security
	seed := crypto.SeedGenWithContext(
		e.KeySignature, byteVal, e.SegmentIndex,
		e.BeatPosition, e.PhraseEnergy, e.LastGesture,
	)
	// Increased base intensity for more aggressive, Sid Wilson-like style
	intensity := 0.45 + 0.8*float64(byteVal&0x0f)/15.0
	context := e.GetPerformanceContext()
	policy := gesture.ComputeGesturePolicy(seed, byteVal, intensity, context)

	// reuse per-engine buffer to reduce allocations
	if cap(e.SegmentBuf) < eventLen {
		e.SegmentBuf = make([]float64, eventLen)
	}
	segment := e.SegmentBuf[:eventLen]
	pitchShift := 0.8 + 0.5*float64(int(byteVal&0x0f))/15.0
	targetFader := 0.4 + 0.6*policy.Intensity

	// local copies to reduce repeated field access
	srcLen := len(source)
	statePos := e.State.Position
	stateVel := e.State.Velocity
	physics := e.Physics

	invDen := 0.0
	if eventLen > 1 {
		invDen = 1.0 / float64(eventLen-1)
	}

	gateMod := policy.GateModulation

	for i := 0; i < eventLen; i++ {
		var t float64
		if invDen > 0 {
			t = float64(i) * invDen
		} else {
			t = 0.0
		}

		rawVel := policy.VelocityCurve(t, policy.Intensity)
		targetVel := rawVel * pitchShift
		if physics != nil {
			targetVel = physics.ApplyPlatterPhysics(targetVel, 0.04)
			targetVel -= physics.ApplyStylusDrag(0.25)
		}

		stateVel += clamp(targetVel-stateVel, -0.18, 0.18)
		statePos = audio.WrapPosition(statePos+stateVel, srcLen)

		sample := audio.SampleAt(source, statePos)

		gate := policy.GatingCurve(t, policy.Intensity) * gateMod
		if physics != nil {
			e.CrossfaderPos = physics.UpdateCrossfader(targetFader)
		} else {
			e.CrossfaderPos = targetFader
		}
		smoothedGate := smoothGate(gate*e.CrossfaderPos, t)

		segment[i] = sample * smoothedGate
		e.State.LastGate = gate
	}

	// write back optimized locals
	e.State.Position = statePos
	e.State.Velocity = stateVel

	e.LastGesture = policy.Type
	e.LastIntensity = policy.Intensity
	e.updatePhrase(byteVal, eventLen)
	e.SegmentIndex++
	return segment
}

func (e *Engine) updatePhrase(byteVal byte, eventLen int) {
	beatSamples := float64(audio.SampleRate) * 60.0 / e.Tempo
	beats := float64(eventLen) / beatSamples
	e.BeatPosition += beats
	if e.BeatPosition >= 4.0 {
		e.BeatPosition = math.Mod(e.BeatPosition, 4.0)
	}

	targetEnergy := 0.3 + 0.7*float64(byteVal&0x0f)/15.0
	e.PhraseEnergy += (targetEnergy - e.PhraseEnergy) * 0.12
	if e.PhraseEnergy < 0.1 {
		e.PhraseEnergy = 0.1
	} else if e.PhraseEnergy > 1.0 {
		e.PhraseEnergy = 1.0
	}
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
