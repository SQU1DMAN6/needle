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
	SegmentIndex  int64
	Tempo         float64
	BeatPosition  float64
	PhraseEnergy  float64
	CrossfaderPos float64
	LastGesture   int
	LastIntensity float64
	Physics       *PhysicsState
	SegmentBuf    []float64
}

func NewEngine(keyData []float64, segmentLen int) *Engine {
	keySig := crypto.KeySignature(keyData)
	tempo := 88.0 + float64((keySig>>32)%32)
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

func (e *Engine) EventLen(byteVal byte) int {
	beatSamples := float64(audio.SampleRate) * 60.0 / e.Tempo
	subdivisions := []float64{0.55, 0.65, 0.8, 1.0}

	seed := crypto.SeedGen(e.KeySignature, byteVal, e.SegmentIndex)
	rng := crypto.NewPRNG(seed)
	sub := subdivisions[rng.NextInt(len(subdivisions))]
	drift := (rng.Next() - 0.5) * 0.08 * beatSamples

	phraseFactor := 1.0
	if e.BeatPosition > 3.0 {
		phraseFactor = 1.15
	}

	length := int(sub*beatSamples*phraseFactor + drift)
	minLen := int(math.Round(0.32 * float64(audio.SampleRate)))
	maxLen := int(math.Round(0.42 * float64(audio.SampleRate)))
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

func (e *Engine) SynthesizeEvent(source []float64, byteVal byte, final bool) []float64 {
	seed := crypto.SeedGenWithContext(
		e.KeySignature, byteVal, e.SegmentIndex,
		e.BeatPosition, e.PhraseEnergy, e.LastGesture,
	)
	nibbleAccent := float64(byteVal&0x0f) / 15.0
	intensity := 0.42 + 0.42*nibbleAccent + 0.12*e.PhraseEnergy
	if e.BeatPosition < 0.18 || e.BeatPosition > 3.65 {
		intensity += 0.08
	}
	if intensity > 0.96 {
		intensity = 0.96
	}
	context := e.GetPerformanceContext()
	policy := gesture.ComputeGesturePolicy(seed, byteVal, intensity, context)

	return e.synthesizeWithPolicy(source, byteVal, policy, final)
}

func (e *Engine) SynthesizeEventWithTechnique(source []float64, byteVal byte, techniqueID int, final bool) []float64 {
	nibbleAccent := float64(byteVal&0x0f) / 15.0
	intensity := 0.42 + 0.42*nibbleAccent + 0.12*e.PhraseEnergy
	if e.BeatPosition < 0.18 || e.BeatPosition > 3.65 {
		intensity += 0.08
	}
	if intensity > 0.96 {
		intensity = 0.96
	}

	eventLen := e.deterministicEventLen(byteVal)

	policy := &gesture.GesturePolicy{
		Type:         techniqueID,
		Intensity:    intensity,
		DurationMult: 1.0,
	}

	tmpl := gesture.Templates[techniqueID]
	if tmpl != nil {
		policy.VelocityCurve = tmpl.VelocityCurve
		policy.GatingCurve = tmpl.GatingCurve
	} else {
		tmpl0 := gesture.Templates[0]
		if tmpl0 != nil {
			policy.VelocityCurve = tmpl0.VelocityCurve
			policy.GatingCurve = tmpl0.GatingCurve
		} else {
			policy.VelocityCurve = func(t, i float64) float64 { return 0.8 }
			policy.GatingCurve = func(t, i float64) float64 { return 1.0 }
		}
	}

	policy.GateModulation = 1.0

	return e.synthesizeWithPolicyExplicit(source, byteVal, policy, eventLen, final)
}

func (e *Engine) deterministicEventLen(byteVal byte) int {
	beatSamples := float64(audio.SampleRate) * 60.0 / e.Tempo
	nibble := float64(byteVal&0x0f) / 15.0
	sub := 0.55 + 0.45*nibble
	phraseFactor := 1.0
	if e.BeatPosition > 3.0 {
		phraseFactor = 1.15
	}
	length := int(sub * beatSamples * phraseFactor)
	minLen := int(math.Round(0.32 * float64(audio.SampleRate)))
	maxLen := int(math.Round(0.42 * float64(audio.SampleRate)))
	if length < minLen {
		length = minLen
	}
	if length > maxLen {
		length = maxLen
	}
	return length
}

func (e *Engine) synthesizeWithPolicy(source []float64, byteVal byte, policy *gesture.GesturePolicy, final bool) []float64 {
	eventLen := e.EventLen(byteVal)
	return e.synthesizeWithPolicyExplicit(source, byteVal, policy, eventLen, final)
}

func (e *Engine) synthesizeWithPolicyExplicit(source []float64, byteVal byte, policy *gesture.GesturePolicy, eventLen int, final bool) []float64 {
	if cap(e.SegmentBuf) < eventLen {
		e.SegmentBuf = make([]float64, eventLen)
	}
	segment := e.SegmentBuf[:eventLen]
	pitchShift := 0.8 + 0.5*float64(int(byteVal&0x0f))/15.0
	targetFader := 0.4 + 0.6*policy.Intensity

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
		if final {
			fadeStart := 0.65
			if t > fadeStart {
				fade := 1.0 - ((t - fadeStart) / (1.0 - fadeStart))
				if fade < 0 {
					fade = 0
				}
				smoothedGate *= 0.5 + 0.5*fade
			}
		}

		segment[i] = sample * smoothedGate
		e.State.LastGate = gate
	}

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

func (e *Engine) SynthesizeSegment(source []float64, byteVal byte, position int64) []float64 {
	return e.SynthesizeEvent(source, byteVal, false)
}

func (e *Engine) Reset() {
	e.State = &State{Position: 0, Velocity: 0, LastGate: 1.0}
	e.SegmentIndex = 0
	e.BeatPosition = 0.0
	e.PhraseEnergy = 0.5
	e.CrossfaderPos = 0.5
	e.LastGesture = -1
	e.LastIntensity = 0.6
}

func (e *Engine) ResetKeepingContext() {
	e.State = &State{Position: 0, Velocity: 0, LastGate: 1.0}
	e.SegmentIndex = 0
	e.BeatPosition = 0.0
	e.PhraseEnergy = 0.5
	e.CrossfaderPos = 0.5
	e.LastGesture = -1
	e.LastIntensity = 0.6
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