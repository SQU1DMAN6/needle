package motion

import "math"

// PhysicsState models realistic turntable physics
type PhysicsState struct {
	// Platter dynamics
	PlatterVelocity float64 // Rotational velocity (normalized -1 to 1)
	PlatterInertia  float64 // Momentum resistance
	PlatterFriction float64 // Natural friction coefficient

	// Stylus/Needle dynamics
	StylusDrag  float64 // Friction from stylus contact
	StylusForce float64 // Normal force (0-1)

	// Crossfader state
	CrossfaderPos    float64 // 0.0 (left) to 1.0 (right)
	CrossfaderSmooth float64 // Smoothing factor for response

	// Motor dynamics
	MotorTorque  float64 // Motor assist torque
	MotorInertia float64 // Motor responsiveness

	// Historical context
	PreviousVelocity float64
	AccelerationRate float64
}

// NewPhysicsState creates a physics state with realistic defaults
func NewPhysicsState() *PhysicsState {
	return &PhysicsState{
		PlatterVelocity:  0.0,
		PlatterInertia:   0.92, // 92% momentum retention (smooth, realistic)
		PlatterFriction:  0.08, // 8% natural friction per sample
		StylusDrag:       0.0,
		StylusForce:      0.5,
		CrossfaderPos:    0.5,
		CrossfaderSmooth: 0.15,
		MotorTorque:      0.0,
		MotorInertia:     0.88,
		PreviousVelocity: 0.0,
		AccelerationRate: 0.0,
	}
}

// ApplyPlatterPhysics simulates platter inertia, friction, and motor effects
// Returns the actual platter velocity after physics simulation
func (ps *PhysicsState) ApplyPlatterPhysics(targetVel float64, motorForce float64) float64 {
	// Motor assistance
	ps.MotorTorque = motorForce * 0.15

	// Calculate acceleration needed
	velocityError := targetVel - ps.PlatterVelocity
	acceleration := velocityError * 0.12 // Response speed

	// Apply motor assist to acceleration
	acceleration = (acceleration + ps.MotorTorque) * ps.MotorInertia

	// Clamp acceleration to realistic limits
	if acceleration > 0.15 {
		acceleration = 0.15
	} else if acceleration < -0.15 {
		acceleration = -0.15
	}

	ps.AccelerationRate = acceleration
	ps.PreviousVelocity = ps.PlatterVelocity

	// Apply acceleration to velocity
	ps.PlatterVelocity += acceleration

	// Apply platter inertia (momentum retention)
	ps.PlatterVelocity *= ps.PlatterInertia

	// Apply natural friction damping
	if math.Abs(ps.PlatterVelocity) > 0.001 {
		if ps.PlatterVelocity > 0 {
			ps.PlatterVelocity -= ps.PlatterFriction * 0.02
		} else {
			ps.PlatterVelocity += ps.PlatterFriction * 0.02
		}
	}

	// Clamp final velocity
	if ps.PlatterVelocity > 1.0 {
		ps.PlatterVelocity = 1.0
	} else if ps.PlatterVelocity < -1.0 {
		ps.PlatterVelocity = -1.0
	}

	return ps.PlatterVelocity
}

// ApplyStylusDrag simulates stylus friction effects
func (ps *PhysicsState) ApplyStylusDrag(contactForce float64) float64 {
	// Stylus drag proportional to normal force
	dragFactor := contactForce * 0.08
	if ps.PlatterVelocity > 0 {
		ps.StylusDrag = dragFactor
	} else {
		ps.StylusDrag = -dragFactor
	}
	return ps.StylusDrag
}

// UpdateCrossfader smoothly updates crossfader position with realistic response
func (ps *PhysicsState) UpdateCrossfader(targetPos float64) float64 {
	posError := targetPos - ps.CrossfaderPos
	// Smooth response with resistance
	smoothing := ps.CrossfaderSmooth
	ps.CrossfaderPos += posError * smoothing
	return ps.CrossfaderPos
}

// GetMomentumInfluence returns how much previous momentum affects current motion
func (ps *PhysicsState) GetMomentumInfluence() float64 {
	// Momentum carries over, creating smooth transitions
	// This makes gestures blend together naturally
	return math.Abs(ps.PreviousVelocity) * 0.4
}

// ApplyJitterNoise adds realistic microscopic vibration
func (ps *PhysicsState) ApplyJitterNoise(frequency float64, phase float64, magnitude float64) float64 {
	// Simulate needle jitter and platter wobble
	jitter := math.Sin(phase*frequency) * magnitude
	return jitter * (1.0 - math.Abs(ps.PlatterVelocity)*0.3) // Reduce jitter at high speeds
}

// SimulateSpindown gradually reduces velocity (like motor cutoff)
func (ps *PhysicsState) SimulateSpindown(duration float64, t float64) float64 {
	if t < duration {
		decayRate := 1.0 - (t / duration)
		return ps.PlatterVelocity * decayRate
	}
	return 0.0
}

// GetEffectiveVelocity returns velocity modified by all physical effects
func (ps *PhysicsState) GetEffectiveVelocity() float64 {
	effective := ps.PlatterVelocity - ps.StylusDrag
	return effective
}
