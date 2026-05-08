package gesture

import (
	"math"

	"needle/internal/crypto"
)

// GesturePolicy defines how a gesture should be synthesized
type GesturePolicy struct {
	Type          int
	VelocityCurve func(t, intensity float64) float64
	GatingCurve   func(t, intensity float64) float64
	Intensity     float64
	DurationMult  float64
}

// ComputeGesturePolicy generates a gesture policy from a seed
func ComputeGesturePolicy(seed uint64, byteVal byte, baseIntensity float64) *GesturePolicy {
	rng := crypto.NewPRNG(seed)

	gestureType := rng.NextInt(32) // Expanded from 16 to 32 for better discrimination
	intensityVar := rng.NextFloat(0.5, 1.5)
	durationVar := rng.NextFloat(0.8, 1.2)

	intensity := baseIntensity * intensityVar
	if intensity > 1.0 {
		intensity = 1.0
	}

	base := 0.6 + 0.6*intensity

	policy := &GesturePolicy{
		Type:         gestureType,
		Intensity:    intensity,
		DurationMult: durationVar,
	}

	switch gestureType {
	case 0: // Forward drag with modulation
		policy.VelocityCurve = func(t, i float64) float64 {
			mod := math.Sin(2*math.Pi*t) * 0.2 * i
			return (base * (0.8 + 0.2*math.Sin(2*math.Pi*t))) + mod
		}
		policy.GatingCurve = func(t, i float64) float64 { return 1.0 }

	case 1: // Reverse pull with decay
		policy.VelocityCurve = func(t, i float64) float64 {
			return -base * (0.85 - 0.45*t)
		}
		policy.GatingCurve = func(t, i float64) float64 { return 1.0 }

	case 2: // Baby scratch (alternating)
		policy.VelocityCurve = func(t, i float64) float64 {
			omega := 20.0 + 10.0*i
			return base * math.Sin(omega*math.Pi*t)
		}
		policy.GatingCurve = func(t, i float64) float64 { return 0.95 }

	case 3: // Transformer cut (pulsing)
		policy.VelocityCurve = func(t, i float64) float64 { return base * 0.8 }
		policy.GatingCurve = func(t, i float64) float64 {
			pulses := 5 + int(i*5)
			phase := t * float64(pulses)
			if math.Mod(phase, 1.0) < 0.5 {
				return 1.0
			}
			return 0.0
		}

	case 4: // Tape stop (smooth deceleration)
		policy.VelocityCurve = func(t, i float64) float64 {
			return base * (1.0 - 0.9*t)
		}
		policy.GatingCurve = func(t, i float64) float64 { return 1.0 }

	case 5: // Spinback (rapid reverse)
		policy.VelocityCurve = func(t, i float64) float64 {
			if t < 0.25 {
				return -base * 1.5
			}
			return base * 0.9
		}
		policy.GatingCurve = func(t, i float64) float64 { return 1.0 }

	case 6: // Jitter scratch (unstable motion)
		policy.VelocityCurve = func(t, i float64) float64 {
			noise := 0.15 * math.Sin(30*t*math.Pi) * i
			return base*(1.0+noise) + base*0.1*math.Sin(50*t*math.Pi)
		}
		policy.GatingCurve = func(t, i float64) float64 { return 1.0 }

	case 7: // Stutter scratch (high-frequency gating)
		policy.VelocityCurve = func(t, i float64) float64 {
			freq := 30.0 + 30.0*i
			return base * math.Sin(2*math.Pi*freq*t)
		}
		policy.GatingCurve = func(t, i float64) float64 {
			pulses := 6 + int(i*6)
			phase := t * float64(pulses)
			if math.Mod(phase, 1.0) < 0.5 {
				return 1.0
			}
			return 0.0
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

	case 11: // High-frequency oscillation
		policy.VelocityCurve = func(t, i float64) float64 {
			return base * math.Sin(2*math.Pi*(4.0+i*4.0)*t)
		}
		policy.GatingCurve = func(t, i float64) float64 { return 0.9 }

	case 12: // Complex gating with pitch
		policy.VelocityCurve = func(t, i float64) float64 {
			return base * (0.4 + 0.6*math.Sin(6*math.Pi*t))
		}
		policy.GatingCurve = func(t, i float64) float64 {
			pulses := 4 + int(i*4)
			phase := t * float64(pulses)
			if math.Mod(phase, 1.0) < 0.5 {
				return 1.0
			}
			return 0.0
		}

	case 13: // Amplitude modulation with gating
		policy.VelocityCurve = func(t, i float64) float64 {
			return base * (0.8 + 0.4*math.Sin(8*math.Pi*t))
		}
		policy.GatingCurve = func(t, i float64) float64 {
			return 0.8 + 0.2*math.Sin(4*math.Pi*t)
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

	case 18: // Fast forward scratch
		policy.VelocityCurve = func(t, i float64) float64 {
			omega := 25.0 + 15.0*i
			return base * 1.3 * math.Sin(omega*math.Pi*t)
		}
		policy.GatingCurve = func(t, i float64) float64 { return 0.9 }

	case 19: // Double pulse gating
		policy.VelocityCurve = func(t, i float64) float64 { return base * 0.85 }
		policy.GatingCurve = func(t, i float64) float64 {
			phase := math.Mod(t*8, 1.0)
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
			if t < 0.3 {
				return -base * 1.6
			}
			return base * 0.7 * math.Cos(2*math.Pi*(t-0.3))
		}
		policy.GatingCurve = func(t, i float64) float64 { return 1.0 }

	case 22: // Triple jitter
		policy.VelocityCurve = func(t, i float64) float64 {
			n1 := 0.1 * math.Sin(35*t*math.Pi) * i
			n2 := 0.08 * math.Cos(55*t*math.Pi) * i
			return base*(1.0+n1+n2) + base*0.12*math.Sin(75*t*math.Pi)
		}
		policy.GatingCurve = func(t, i float64) float64 { return 1.0 }

	case 23: // Fast flutter gating
		policy.VelocityCurve = func(t, i float64) float64 {
			freq := 35.0 + 35.0*i
			return base * 0.9 * math.Sin(2*math.Pi*freq*t)
		}
		policy.GatingCurve = func(t, i float64) float64 {
			pulses := 8 + int(i*8)
			phase := t * float64(pulses)
			frac := math.Mod(phase, 1.0)
			if frac < 0.3 {
				return 1.0
			}
			return 0.2
		}

	case 24: // Sawtooth sweep
		policy.VelocityCurve = func(t, i float64) float64 {
			return base * (1.0 - 2.0*(t-0.5)*(t-0.5))
		}
		policy.GatingCurve = func(t, i float64) float64 { return 1.0 }

	case 25: // Reverse with multi-gating
		policy.VelocityCurve = func(t, i float64) float64 {
			return -base * (0.5 + 0.5*t) * (1.0 + 0.3*math.Sin(3*math.Pi*t))
		}
		policy.GatingCurve = func(t, i float64) float64 {
			return 0.8 + 0.2*math.Sin(6*math.Pi*t)
		}

	case 26: // Power decay with ripple
		policy.VelocityCurve = func(t, i float64) float64 {
			return base * math.Pow((1.0-t), 1.2) * (1.0 + 0.15*math.Cos(5*math.Pi*t))
		}
		policy.GatingCurve = func(t, i float64) float64 { return 1.0 }

	case 27: // Fast dual oscillation
		policy.VelocityCurve = func(t, i float64) float64 {
			f1 := 5.0 + 3.0*i
			f2 := 12.0 + 5.0*i
			return base * (0.6*math.Sin(2*math.Pi*f1*t) + 0.4*math.Cos(2*math.Pi*f2*t))
		}
		policy.GatingCurve = func(t, i float64) float64 { return 0.85 }

	case 28: // Complex reverse with envelope
		policy.VelocityCurve = func(t, i float64) float64 {
			return -base * (0.8 + 0.3*math.Cos(4*math.Pi*t)) * (1.0 - 0.3*t)
		}
		policy.GatingCurve = func(t, i float64) float64 { return 1.0 }

	case 29: // Asymmetric pulse gating
		policy.VelocityCurve = func(t, i float64) float64 { return base * 0.95 }
		policy.GatingCurve = func(t, i float64) float64 {
			phase := math.Mod(t*6, 1.0)
			if phase < 0.35 {
				return 1.0
			}
			return 0.1
		}

	case 30: // Rapid oscillation with decay
		policy.VelocityCurve = func(t, i float64) float64 {
			freq := 15.0 + 20.0*i
			return base * (1.0 - 0.7*t) * math.Sin(2*math.Pi*freq*t)
		}
		policy.GatingCurve = func(t, i float64) float64 { return 0.9 }

	case 31: // Reverse triple modulation
		policy.VelocityCurve = func(t, i float64) float64 {
			m1 := 0.2 * math.Sin(3*math.Pi*t) * i
			m2 := 0.15 * math.Cos(5*math.Pi*t) * i
			return -base * (0.6 + m1 + m2)
		}
		policy.GatingCurve = func(t, i float64) float64 {
			return 0.9 + 0.1*math.Sin(4*math.Pi*t)
		}

	default:
		policy.VelocityCurve = func(t, i float64) float64 { return base }
		policy.GatingCurve = func(t, i float64) float64 { return 1.0 }
	}

	return policy
}
