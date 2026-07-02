package gesture

import (
	"math"
)

// CurveTemplate defines canonical velocity and gating curves for a technique.
// These curves are deterministic functions of (t, intensity) only —
// no PRNG, no context state, no runtime variation.
type CurveTemplate struct {
	TechniqueID   int
	VelocityCurve func(t, intensity float64) float64
	GatingCurve   func(t, intensity float64) float64
	Direction     int // +1 forward, -1 reverse, 0 alternating
}

// Templates is the canonical registry of all 32 gesture curve templates.
// Indexed by technique_id (0-31).
var Templates [32]*CurveTemplate

func init() {
	initTemplates()
}

func initTemplates() {
	for i := range Templates {
		Templates[i] = nil // will be set below
	}

	Templates[0] = &CurveTemplate{
		TechniqueID: 0,
		Direction:   +1,
		VelocityCurve: func(t, i float64) float64 {
			base := 0.72 + 0.62*i
			mod := math.Sin(2*math.Pi*t) * 0.15 * i
			wobble := 0.05 * math.Sin(8*math.Pi*t) * i
			return (base * (0.75 + 0.25*math.Sin(2*math.Pi*t))) + mod + wobble
		},
		GatingCurve: func(t, i float64) float64 {
			return 0.95 + 0.05*math.Sin(4*math.Pi*t)
		},
	}

	Templates[1] = &CurveTemplate{
		TechniqueID: 1,
		Direction:   -1,
		VelocityCurve: func(t, i float64) float64 {
			base := 0.72 + 0.62*i
			decel := 0.85 - 0.35*t*t
			return -base * (decel + 0.1*i*math.Cos(3*math.Pi*t))
		},
		GatingCurve: func(t, i float64) float64 {
			return 0.97 + 0.03*math.Sin(2*math.Pi*t)
		},
	}

	Templates[2] = &CurveTemplate{
		TechniqueID: 2,
		Direction:   0,
		VelocityCurve: func(t, i float64) float64 {
			base := 0.72 + 0.62*i
			omega := 3.5 + 1.5*i
			envelope := 1.0 - 0.15*math.Pow(t, 1.5)
			return base * envelope * math.Sin(omega*math.Pi*t)
		},
		GatingCurve: func(t, i float64) float64 {
			return 0.95 + 0.05*math.Cos(math.Pi*t)
		},
	}

	Templates[3] = &CurveTemplate{
		TechniqueID: 3,
		Direction:   +1,
		VelocityCurve: func(t, i float64) float64 {
			base := 0.72 + 0.62*i
			return base * (0.7 + 0.3*math.Cos(2*math.Pi*t))
		},
		GatingCurve: func(t, i float64) float64 {
			cuts := 2 + int(i*2)
			phase := t * float64(cuts)
			frac := math.Mod(phase, 1.0)
			if frac < 0.35 {
				return 1.0 * (frac / 0.35)
			} else if frac < 0.65 {
				return 1.0 * (1.0 - (frac-0.35)/0.3)
			}
			return 0.1 * (1.0 - frac) / 0.35
		},
	}

	Templates[4] = &CurveTemplate{
		TechniqueID: 4,
		Direction:   +1,
		VelocityCurve: func(t, i float64) float64 {
			base := 0.72 + 0.62*i
			return base * (1.0 - 0.85*t)
		},
		GatingCurve: func(t, i float64) float64 { return 1.0 },
	}

	Templates[5] = &CurveTemplate{
		TechniqueID: 5,
		Direction:   0,
		VelocityCurve: func(t, i float64) float64 {
			base := 0.72 + 0.62*i
			if t < 0.3 {
				return -base * 1.3
			}
			return base * 0.8
		},
		GatingCurve: func(t, i float64) float64 { return 1.0 },
	}

	Templates[6] = &CurveTemplate{
		TechniqueID: 6,
		Direction:   +1,
		VelocityCurve: func(t, i float64) float64 {
			base := 0.72 + 0.62*i
			noise := 0.12 * math.Sin(8*t*math.Pi) * i
			wobble := 0.08 * math.Sin(12*t*math.Pi)
			return base*(0.9+noise) + wobble
		},
		GatingCurve: func(t, i float64) float64 { return 1.0 },
	}

	Templates[7] = &CurveTemplate{
		TechniqueID: 7,
		Direction:   0,
		VelocityCurve: func(t, i float64) float64 {
			base := 0.72 + 0.62*i
			freq := 5.5 + 2.5*i
			decay := 1.0 - 0.4*t*t
			return base * decay * math.Sin(2*math.Pi*freq*t)
		},
		GatingCurve: func(t, i float64) float64 {
			cuts := 3 + int(i*2)
			phase := t * float64(cuts)
			frac := math.Mod(phase, 1.0)
			if frac < 0.4 {
				return 0.85 + 0.15*math.Sin(math.Pi*frac/0.4)
			}
			return 0.7 * math.Exp(-3*(frac-0.4))
		},
	}

	Templates[8] = &CurveTemplate{
		TechniqueID: 8,
		Direction:   +1,
		VelocityCurve: func(t, i float64) float64 {
			base := 0.72 + 0.62*i
			return base * (0.5 + 0.5*math.Cos(2*math.Pi*t))
		},
		GatingCurve: func(t, i float64) float64 { return 1.0 },
	}

	Templates[9] = &CurveTemplate{
		TechniqueID: 9,
		Direction:   -1,
		VelocityCurve: func(t, i float64) float64 {
			base := 0.72 + 0.62*i
			return -base * (0.4 + 0.6*t)
		},
		GatingCurve: func(t, i float64) float64 {
			return 0.9 + 0.1*math.Cos(4*math.Pi*t)
		},
	}

	Templates[10] = &CurveTemplate{
		TechniqueID: 10,
		Direction:   +1,
		VelocityCurve: func(t, i float64) float64 {
			base := 0.72 + 0.62*i
			return base * (1.2 - 0.8*math.Pow(t, 2))
		},
		GatingCurve: func(t, i float64) float64 { return 1.0 },
	}

	Templates[11] = &CurveTemplate{
		TechniqueID: 11,
		Direction:   0,
		VelocityCurve: func(t, i float64) float64 {
			base := 0.72 + 0.62*i
			return base * math.Sin(2*math.Pi*(2.0+i*1.5)*t)
		},
		GatingCurve: func(t, i float64) float64 { return 0.95 },
	}

	Templates[12] = &CurveTemplate{
		TechniqueID: 12,
		Direction:   +1,
		VelocityCurve: func(t, i float64) float64 {
			base := 0.72 + 0.62*i
			return base * (0.4 + 0.6*math.Sin(3*math.Pi*t))
		},
		GatingCurve: func(t, i float64) float64 {
			pulses := 2 + int(i*2)
			phase := t * float64(pulses)
			if math.Mod(phase, 1.0) < 0.5 {
				return 1.0
			}
			return 0.0
		},
	}

	Templates[13] = &CurveTemplate{
		TechniqueID: 13,
		Direction:   +1,
		VelocityCurve: func(t, i float64) float64 {
			base := 0.72 + 0.62*i
			return base * (0.8 + 0.4*math.Sin(3*math.Pi*t))
		},
		GatingCurve: func(t, i float64) float64 {
			return 0.85 + 0.15*math.Sin(2*math.Pi*t)
		},
	}

	Templates[14] = &CurveTemplate{
		TechniqueID: 14,
		Direction:   -1,
		VelocityCurve: func(t, i float64) float64 {
			base := 0.72 + 0.62*i
			return -base * (0.7 + 0.3*math.Cos(3*math.Pi*t))
		},
		GatingCurve: func(t, i float64) float64 { return 1.0 },
	}

	Templates[15] = &CurveTemplate{
		TechniqueID: 15,
		Direction:   0,
		VelocityCurve: func(t, i float64) float64 {
			base := 0.72 + 0.62*i
			return base * (0.5 + 0.5*math.Sin(2*math.Pi*t)*i)
		},
		GatingCurve: func(t, i float64) float64 {
			return 0.85 + 0.15*math.Cos(5*math.Pi*t)
		},
	}

	Templates[16] = &CurveTemplate{
		TechniqueID: 16,
		Direction:   +1,
		VelocityCurve: func(t, i float64) float64 {
			base := 0.72 + 0.62*i
			m1 := math.Sin(2*math.Pi*t) * 0.15 * i
			m2 := math.Cos(4*math.Pi*t) * 0.1 * i
			return base*(0.7+0.3*m1) + m2
		},
		GatingCurve: func(t, i float64) float64 { return 1.0 },
	}

	Templates[17] = &CurveTemplate{
		TechniqueID: 17,
		Direction:   -1,
		VelocityCurve: func(t, i float64) float64 {
			base := 0.72 + 0.62*i
			return -base * (0.9 - 0.6*t + 0.3*math.Sin(3*math.Pi*t)*i)
		},
		GatingCurve: func(t, i float64) float64 { return 1.0 },
	}

	Templates[18] = &CurveTemplate{
		TechniqueID: 18,
		Direction:   0,
		VelocityCurve: func(t, i float64) float64 {
			base := 0.72 + 0.62*i
			omega := 6.5 + 1.5*i
			return base * 1.2 * math.Sin(omega*math.Pi*t)
		},
		GatingCurve: func(t, i float64) float64 { return 0.92 },
	}

	Templates[19] = &CurveTemplate{
		TechniqueID: 19,
		Direction:   +1,
		VelocityCurve: func(t, i float64) float64 {
			base := 0.72 + 0.62*i
			return base * 0.85
		},
		GatingCurve: func(t, i float64) float64 {
			phase := math.Mod(t*4, 1.0)
			if phase < 0.25 || (phase > 0.5 && phase < 0.75) {
				return 1.0
			}
			return 0.0
		},
	}

	Templates[20] = &CurveTemplate{
		TechniqueID: 20,
		Direction:   +1,
		VelocityCurve: func(t, i float64) float64 {
			base := 0.72 + 0.62*i
			return base * math.Pow((1.0-t), 1.5) * (1.0 + 0.2*i)
		},
		GatingCurve: func(t, i float64) float64 { return 1.0 },
	}

	Templates[21] = &CurveTemplate{
		TechniqueID: 21,
		Direction:   0,
		VelocityCurve: func(t, i float64) float64 {
			base := 0.72 + 0.62*i
			if t < 0.35 {
				return -base * 1.4
			}
			return base * 0.6 * math.Cos(2*math.Pi*(t-0.35))
		},
		GatingCurve: func(t, i float64) float64 { return 1.0 },
	}

	Templates[22] = &CurveTemplate{
		TechniqueID: 22,
		Direction:   0,
		VelocityCurve: func(t, i float64) float64 {
			base := 0.72 + 0.62*i
			n1 := 0.1 * math.Sin(10*t*math.Pi) * i * (1.0 - 0.3*t)
			n2 := 0.06 * math.Cos(12*t*math.Pi) * i
			return base*(0.95+n1+n2) + base*0.08*math.Sin(15*t*math.Pi)*(1.0-t)
		},
		GatingCurve: func(t, i float64) float64 {
			cutFreq := 2.5 + 1.5*i
			return 0.85 + 0.15*math.Sin(2*math.Pi*cutFreq*t)
		},
	}

	Templates[23] = &CurveTemplate{
		TechniqueID: 23,
		Direction:   0,
		VelocityCurve: func(t, i float64) float64 {
			base := 0.72 + 0.62*i
			freq := 9.0 + 3.0*i
			return base * 0.9 * math.Sin(2*math.Pi*freq*t)
		},
		GatingCurve: func(t, i float64) float64 {
			pulses := 4 + int(i*2)
			phase := t * float64(pulses)
			frac := math.Mod(phase, 1.0)
			if frac < 0.4 {
				return 1.0
			}
			return 0.25
		},
	}

	Templates[24] = &CurveTemplate{
		TechniqueID: 24,
		Direction:   +1,
		VelocityCurve: func(t, i float64) float64 {
			base := 0.72 + 0.62*i
			return base * (1.0 - 2.0*(t-0.5)*(t-0.5))
		},
		GatingCurve: func(t, i float64) float64 { return 1.0 },
	}

	Templates[25] = &CurveTemplate{
		TechniqueID: 25,
		Direction:   -1,
		VelocityCurve: func(t, i float64) float64 {
			base := 0.72 + 0.62*i
			return -base * (0.5 + 0.5*t) * (1.0 + 0.25*math.Sin(2*math.Pi*t))
		},
		GatingCurve: func(t, i float64) float64 {
			return 0.85 + 0.15*math.Sin(3*math.Pi*t)
		},
	}

	Templates[26] = &CurveTemplate{
		TechniqueID: 26,
		Direction:   +1,
		VelocityCurve: func(t, i float64) float64 {
			base := 0.72 + 0.62*i
			return base * math.Pow((1.0-t), 1.2) * (1.0 + 0.12*math.Cos(3*math.Pi*t))
		},
		GatingCurve: func(t, i float64) float64 { return 1.0 },
	}

	Templates[27] = &CurveTemplate{
		TechniqueID: 27,
		Direction:   0,
		VelocityCurve: func(t, i float64) float64 {
			base := 0.72 + 0.62*i
			f1 := 3.0 + 0.5*i
			f2 := 6.0 + 1.0*i
			envelope := 0.9 + 0.1*math.Sin(2*math.Pi*t)
			return base * envelope * (0.65*math.Sin(2*math.Pi*f1*t) + 0.35*math.Sin(2*math.Pi*f2*t))
		},
		GatingCurve: func(t, i float64) float64 {
			return 0.85 + 0.15*math.Sin(2*math.Pi*t)
		},
	}

	Templates[28] = &CurveTemplate{
		TechniqueID: 28,
		Direction:   -1,
		VelocityCurve: func(t, i float64) float64 {
			base := 0.72 + 0.62*i
			return -base * (0.8 + 0.25*math.Cos(3*math.Pi*t)) * (1.0 - 0.3*t)
		},
		GatingCurve: func(t, i float64) float64 { return 1.0 },
	}

	Templates[29] = &CurveTemplate{
		TechniqueID: 29,
		Direction:   +1,
		VelocityCurve: func(t, i float64) float64 {
			base := 0.72 + 0.62*i
			return base * 0.95
		},
		GatingCurve: func(t, i float64) float64 {
			phase := math.Mod(t*3, 1.0)
			if phase < 0.4 {
				return 1.0
			}
			return 0.1
		},
	}

	Templates[30] = &CurveTemplate{
		TechniqueID: 30,
		Direction:   0,
		VelocityCurve: func(t, i float64) float64 {
			base := 0.72 + 0.62*i
			freq := 5.5 + 2.5*i
			return base * (1.0 - 0.6*t) * math.Sin(2*math.Pi*freq*t)
		},
		GatingCurve: func(t, i float64) float64 { return 0.92 },
	}

	Templates[31] = &CurveTemplate{
		TechniqueID: 31,
		Direction:   -1,
		VelocityCurve: func(t, i float64) float64 {
			base := 0.72 + 0.62*i
			m1 := 0.15 * math.Sin(2*math.Pi*t) * i
			m2 := 0.12 * math.Cos(3*math.Pi*t) * i
			return -base * (0.6 + m1 + m2)
		},
		GatingCurve: func(t, i float64) float64 {
			return 0.92 + 0.08*math.Sin(2*math.Pi*t)
		},
	}
}