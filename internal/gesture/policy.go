package gesture

import (
	"math"

	"needle/internal/crypto"
)

// PerformanceContext exposes performance state to gesture policy
// without creating a circular import dependency.
type PerformanceContext struct {
	PreviousType int
	Momentum     float64
	Energy       float64
	BeatPhase    float64
	Crossfader   float64
}

// GesturePolicy defines how a gesture should be synthesized
type GesturePolicy struct {
	Type           int
	VelocityCurve  func(t, intensity float64) float64
	GatingCurve    func(t, intensity float64) float64
	Intensity      float64
	DurationMult   float64
	GateModulation float64
}

// ComputeGesturePolicy generates a gesture policy from a seed
func ComputeGesturePolicy(seed uint64, byteVal byte, baseIntensity float64, ctx PerformanceContext) *GesturePolicy {
	rng := crypto.NewPRNG(seed)

	gestureType := rng.NextInt(32) // Expanded from 16 to 32 for better discrimination
	if ctx.PreviousType >= 0 {
		gestureType = (gestureType + ctx.PreviousType + int(ctx.Energy*3.0) + int(ctx.BeatPhase*2.0)) % 32
	}
	intensityVar := rng.NextFloat(0.5, 1.5)
	durationVar := rng.NextFloat(0.8, 1.2)

	intensity := baseIntensity * intensityVar * (0.85 + 0.25*ctx.Energy)
	if intensity > 1.0 {
		intensity = 1.0
	}

	// Increase base amplitude for more energetic scratching
	base := 0.8 + 0.8*intensity

	policy := &GesturePolicy{
		Type:         gestureType,
		Intensity:    intensity,
		DurationMult: durationVar,
	}

	// Variety: small random gate modulation to avoid repetition
	gateModulation := 0.85 + 0.30*rng.Next()
	policy.GateModulation = gateModulation

	switch gestureType {
	case 0: // Forward drag with musical modulation
		policy.VelocityCurve = func(t, i float64) float64 {
			// Smooth forward motion with subtle wobble
			mod := math.Sin(2*math.Pi*t) * 0.15 * i
			wobble := 0.05 * math.Sin(8*math.Pi*t) * i
			return (base * (0.75 + 0.25*math.Sin(2*math.Pi*t))) + mod + wobble
		}
		policy.GatingCurve = func(t, i float64) float64 {
			// Smooth crossfader fade with slight flutter
			return 0.95 + 0.05*math.Sin(4*math.Pi*t)
		}

	case 1: // Reverse pull with realistic deceleration
		policy.VelocityCurve = func(t, i float64) float64 {
			// Smooth deceleration curve mimicking platter physics
			decel := 0.85 - 0.35*t*t // Quadratic deceleration
			return -base * (decel + 0.1*i*math.Cos(3*math.Pi*t))
		}
		policy.GatingCurve = func(t, i float64) float64 {
			// Smooth envelope with slight crossfader modulation
			return 0.97 + 0.03*math.Sin(2*math.Pi*t)
		}

	case 2: // Baby scratch (smooth alternating)
		policy.VelocityCurve = func(t, i float64) float64 {
			// Realistic baby scratch with 3-5 Hz oscillation
			omega := 3.5 + 1.5*i // 3.5-5 Hz instead of 18-26 Hz
			envelope := 1.0 - 0.15*math.Pow(t, 1.5)
			return base * envelope * math.Sin(omega*math.Pi*t)
		}
		policy.GatingCurve = func(t, i float64) float64 {
			// Smooth crossfade without harsh staccato
			return 0.95 + 0.05*math.Cos(math.Pi*t)
		}

	case 3: // Transformer cut (realistic crossfader cuts)
		policy.VelocityCurve = func(t, i float64) float64 {
			// Smooth velocity with realistic platter movement
			return base * (0.7 + 0.3*math.Cos(2*math.Pi*t))
		}
		policy.GatingCurve = func(t, i float64) float64 {
			// Smooth crossfader cuts (3-5 cuts, not 4-7 pulses per gesture)
			cuts := 2 + int(i*2) // 2-4 cuts instead of 4-7
			phase := t * float64(cuts)
			frac := math.Mod(phase, 1.0)

			// Smooth attack/release on cuts
			if frac < 0.35 {
				return 1.0 * (frac / 0.35)
			} else if frac < 0.65 {
				return 1.0 * (1.0 - (frac-0.35)/0.3)
			}
			return 0.1 * (1.0 - frac) / 0.35
		}

	case 4: // Tape stop (smooth deceleration)
		policy.VelocityCurve = func(t, i float64) float64 {
			return base * (1.0 - 0.85*t)
		}
		policy.GatingCurve = func(t, i float64) float64 { return 1.0 }

	case 5: // Spinback (rapid reverse)
		policy.VelocityCurve = func(t, i float64) float64 {
			if t < 0.3 {
				return -base * 1.3
			}
			return base * 0.8
		}
		policy.GatingCurve = func(t, i float64) float64 { return 1.0 }

	case 6: // Jitter scratch (unstable motion) - REDUCED FREQUENCY
		policy.VelocityCurve = func(t, i float64) float64 {
			// Reduced from 30-50 Hz to 6-10 Hz for realistic wobble
			noise := 0.12 * math.Sin(8*t*math.Pi) * i
			wobble := 0.08 * math.Sin(12*t*math.Pi)
			return base*(0.9+noise) + wobble
		}
		policy.GatingCurve = func(t, i float64) float64 { return 1.0 }

	case 7: // Crab scratch (high-definition cuts) - REDUCED FREQUENCY
		policy.VelocityCurve = func(t, i float64) float64 {
			// Fast oscillation with natural decay: 5-8 Hz instead of 28-53 Hz
			freq := 5.5 + 2.5*i
			decay := 1.0 - 0.4*t*t
			return base * decay * math.Sin(2*math.Pi*freq*t)
		}
		policy.GatingCurve = func(t, i float64) float64 {
			// Fewer, cleaner cuts: 3-6 instead of 7-12
			cuts := 3 + int(i*2)
			phase := t * float64(cuts)
			frac := math.Mod(phase, 1.0)

			if frac < 0.4 {
				return 0.85 + 0.15*math.Sin(math.Pi*frac/0.4)
			}
			return 0.7 * math.Exp(-3*(frac-0.4))
		}

	case 8: // Cosine sweep
		policy.VelocityCurve = func(t, i float64) float64 {
			return base * (0.5 + 0.5*math.Cos(2*math.Pi*t))
		}
		policy.GatingCurve = func(t, i float64) float64 { return 1.0 }

	case 9: // Reverse with modulated gating
		policy.VelocityCurve = func(t, i float64) float64 {
			return -base * (0.4 + 0.6*t)
		}
		policy.GatingCurve = func(t, i float64) float64 {
			return 0.9 + 0.1*math.Cos(4*math.Pi*t)
		}

	case 10: // Power law velocity
		policy.VelocityCurve = func(t, i float64) float64 {
			return base * (1.2 - 0.8*math.Pow(t, 2))
		}
		policy.GatingCurve = func(t, i float64) float64 { return 1.0 }

	case 11: // High-frequency oscillation - REDUCED
		policy.VelocityCurve = func(t, i float64) float64 {
			// Reduced from 4-8 Hz to 2-4 Hz for less chaotic feel
			return base * math.Sin(2*math.Pi*(2.0+i*1.5)*t)
		}
		policy.GatingCurve = func(t, i float64) float64 { return 0.95 }

	case 12: // Complex gating with pitch - REDUCED
		policy.VelocityCurve = func(t, i float64) float64 {
			// Reduced from potentially 6 Hz to 3-4 Hz
			return base * (0.4 + 0.6*math.Sin(3*math.Pi*t))
		}
		policy.GatingCurve = func(t, i float64) float64 {
			pulses := 2 + int(i*2) // Fewer pulses
			phase := t * float64(pulses)
			if math.Mod(phase, 1.0) < 0.5 {
				return 1.0
			}
			return 0.0
		}

	case 13: // Amplitude modulation with gating - REDUCED
		policy.VelocityCurve = func(t, i float64) float64 {
			// Reduced from 8 Hz to 3-4 Hz
			return base * (0.8 + 0.4*math.Sin(3*math.Pi*t))
		}
		policy.GatingCurve = func(t, i float64) float64 {
			return 0.85 + 0.15*math.Sin(2*math.Pi*t)
		}

	case 14: // Reverse cosine
		policy.VelocityCurve = func(t, i float64) float64 {
			return -base * (0.7 + 0.3*math.Cos(3*math.Pi*t))
		}
		policy.GatingCurve = func(t, i float64) float64 { return 1.0 }

	case 15: // Complex motion with intensity-dependent modulation
		policy.VelocityCurve = func(t, i float64) float64 {
			return base * (0.5 + 0.5*math.Sin(2*math.Pi*t)*i)
		}
		policy.GatingCurve = func(t, i float64) float64 {
			return 0.85 + 0.15*math.Cos(5*math.Pi*t)
		}

	case 16: // Forward with triple modulation
		policy.VelocityCurve = func(t, i float64) float64 {
			m1 := math.Sin(2*math.Pi*t) * 0.15 * i
			m2 := math.Cos(4*math.Pi*t) * 0.1 * i
			return base*(0.7+0.3*m1) + m2
		}
		policy.GatingCurve = func(t, i float64) float64 { return 1.0 }

	case 17: // Backward with complex envelope
		policy.VelocityCurve = func(t, i float64) float64 {
			return -base * (0.9 - 0.6*t + 0.3*math.Sin(3*math.Pi*t)*i)
		}
		policy.GatingCurve = func(t, i float64) float64 { return 1.0 }

	case 18: // Fast forward scratch - REDUCED
		policy.VelocityCurve = func(t, i float64) float64 {
			// Reduced from 25-40 Hz to 6-8 Hz
			omega := 6.5 + 1.5*i
			return base * 1.2 * math.Sin(omega*math.Pi*t)
		}
		policy.GatingCurve = func(t, i float64) float64 { return 0.92 }

	case 19: // Double pulse gating
		policy.VelocityCurve = func(t, i float64) float64 { return base * 0.85 }
		policy.GatingCurve = func(t, i float64) float64 {
			phase := math.Mod(t*4, 1.0) // Reduced from 8 to 4
			if phase < 0.25 || (phase > 0.5 && phase < 0.75) {
				return 1.0
			}
			return 0.0
		}

	case 20: // Smooth deceleration with curve
		policy.VelocityCurve = func(t, i float64) float64 {
			return base * math.Pow((1.0-t), 1.5) * (1.0 + 0.2*i)
		}
		policy.GatingCurve = func(t, i float64) float64 { return 1.0 }

	case 21: // Fast reverse with bounce
		policy.VelocityCurve = func(t, i float64) float64 {
			if t < 0.35 {
				return -base * 1.4
			}
			return base * 0.6 * math.Cos(2*math.Pi*(t-0.35))
		}
		policy.GatingCurve = func(t, i float64) float64 { return 1.0 }

	case 22: // Scribble scratch (complex modulation) - REDUCED
		policy.VelocityCurve = func(t, i float64) float64 {
			// Reduced from 32/48/60 Hz to 8/12/15 Hz
			n1 := 0.1 * math.Sin(10*t*math.Pi) * i * (1.0 - 0.3*t)
			n2 := 0.06 * math.Cos(12*t*math.Pi) * i
			return base*(0.95+n1+n2) + base*0.08*math.Sin(15*t*math.Pi)*(1.0-t)
		}
		policy.GatingCurve = func(t, i float64) float64 {
			// Reduced from 5-8 Hz to 2-4 Hz
			cutFreq := 2.5 + 1.5*i
			return 0.85 + 0.15*math.Sin(2*math.Pi*cutFreq*t)
		}

	case 23: // Fast flutter gating - REDUCED
		policy.VelocityCurve = func(t, i float64) float64 {
			// Reduced from 35-70 Hz to 8-12 Hz
			freq := 9.0 + 3.0*i
			return base * 0.9 * math.Sin(2*math.Pi*freq*t)
		}
		policy.GatingCurve = func(t, i float64) float64 {
			// Reduced pulses from 8-16 to 4-6
			pulses := 4 + int(i*2)
			phase := t * float64(pulses)
			frac := math.Mod(phase, 1.0)
			if frac < 0.4 {
				return 1.0
			}
			return 0.25
		}

	case 24: // Sawtooth sweep
		policy.VelocityCurve = func(t, i float64) float64 {
			return base * (1.0 - 2.0*(t-0.5)*(t-0.5))
		}
		policy.GatingCurve = func(t, i float64) float64 { return 1.0 }

	case 25: // Reverse with multi-gating
		policy.VelocityCurve = func(t, i float64) float64 {
			return -base * (0.5 + 0.5*t) * (1.0 + 0.25*math.Sin(2*math.Pi*t))
		}
		policy.GatingCurve = func(t, i float64) float64 {
			return 0.85 + 0.15*math.Sin(3*math.Pi*t)
		}

	case 26: // Power decay with ripple
		policy.VelocityCurve = func(t, i float64) float64 {
			return base * math.Pow((1.0-t), 1.2) * (1.0 + 0.12*math.Cos(3*math.Pi*t))
		}
		policy.GatingCurve = func(t, i float64) float64 { return 1.0 }

	case 27: // Orbit scratch (smooth multi-layer oscillation)
		policy.VelocityCurve = func(t, i float64) float64 {
			// Reduced from 5-6 Hz + 12-16 Hz to 2.5-3.5 Hz + 5-7 Hz
			f1 := 3.0 + 0.5*i
			f2 := 6.0 + 1.0*i
			envelope := 0.9 + 0.1*math.Sin(2*math.Pi*t)
			return base * envelope * (0.65*math.Sin(2*math.Pi*f1*t) + 0.35*math.Sin(2*math.Pi*f2*t))
		}
		policy.GatingCurve = func(t, i float64) float64 {
			return 0.85 + 0.15*math.Sin(2*math.Pi*t)
		}

	case 28: // Complex reverse with envelope
		policy.VelocityCurve = func(t, i float64) float64 {
			return -base * (0.8 + 0.25*math.Cos(3*math.Pi*t)) * (1.0 - 0.3*t)
		}
		policy.GatingCurve = func(t, i float64) float64 { return 1.0 }

	case 29: // Asymmetric pulse gating
		policy.VelocityCurve = func(t, i float64) float64 { return base * 0.95 }
		policy.GatingCurve = func(t, i float64) float64 {
			phase := math.Mod(t*3, 1.0) // Reduced from 6 to 3
			if phase < 0.4 {
				return 1.0
			}
			return 0.1
		}

	case 30: // Rapid oscillation with decay - REDUCED
		policy.VelocityCurve = func(t, i float64) float64 {
			// Reduced from 15-35 Hz to 5-8 Hz
			freq := 5.5 + 2.5*i
			return base * (1.0 - 0.6*t) * math.Sin(2*math.Pi*freq*t)
		}
		policy.GatingCurve = func(t, i float64) float64 { return 0.92 }

	case 31: // Reverse triple modulation
		policy.VelocityCurve = func(t, i float64) float64 {
			m1 := 0.15 * math.Sin(2*math.Pi*t) * i
			m2 := 0.12 * math.Cos(3*math.Pi*t) * i
			return -base * (0.6 + m1 + m2)
		}
		policy.GatingCurve = func(t, i float64) float64 {
			return 0.92 + 0.08*math.Sin(2*math.Pi*t)
		}

	default:
		policy.VelocityCurve = func(t, i float64) float64 { return base }
		policy.GatingCurve = func(t, i float64) float64 { return 1.0 }
	}

	return policy
}
