package tcf

import (
	"crypto/sha256"
	"encoding/binary"
	"fmt"
	"math"

	"needle/internal/motion"
)

// FaderCurveEnum defines the type of crossfader curve for a TCF gesture.
type FaderCurveEnum int

const (
	FaderSmooth    FaderCurveEnum = 0
	FaderCut       FaderCurveEnum = 1
	FaderFade      FaderCurveEnum = 2
	FaderFlutter   FaderCurveEnum = 3
	FaderPulse     FaderCurveEnum = 4
	FaderConstant  FaderCurveEnum = 5
)

// DirectionEnum defines platter motion direction.
type DirectionEnum int

const (
	DirForward     DirectionEnum = 0
	DirReverse     DirectionEnum = 1
	DirAlternating DirectionEnum = 2
)

// TCF is the Technique Canonical Form — an immutable intermediate representation
// for a single scratching gesture.
type TCF struct {
	TechniqueID uint16      `json:"technique_id"`
	Params      TCParams    `json:"params"`
	ContextHash [32]byte    `json:"-"`
}

// TCParams contains all canonical parameters for a TCF gesture.
type TCParams struct {
	TimeStart       int64          `json:"time_start"`
	Duration        int64          `json:"duration"`         // in samples
	FaderCurve      FaderCurveEnum `json:"fader_curve"`
	PlatterVelocity int32          `json:"platter_velocity"` // fixed-point: Q16.16
	Direction       DirectionEnum  `json:"direction"`
	Intensity       int32          `json:"intensity"` // fixed-point: Q16.16 (0-1.0)
}

// FixedPoint conversion helpers.
func FloatToFixed(f float64) int32 {
	return int32(f * 65536.0)
}

func FixedToFloat(f int32) float64 {
	return float64(f) / 65536.0
}

// SetContextHash computes and stores the SHA256 hash of the audio window
// that this TCF was derived from.
func (t *TCF) SetContextHash(audioWindow []float64) {
	h := sha256.New()
	buf := make([]byte, 8)
	for _, v := range audioWindow {
		bits := math.Float64bits(v)
		binary.BigEndian.PutUint64(buf, bits)
		h.Write(buf)
	}
	copy(t.ContextHash[:], h.Sum(nil))
}

// TCFList is a sortable slice of TCFs for deterministic ordering.
type TCFList []TCF

func (l TCFList) Len() int           { return len(l) }
func (l TCFList) Less(i, j int) bool { return l[i].TechniqueID < l[j].TechniqueID }
func (l TCFList) Swap(i, j int)      { l[i], l[j] = l[j], l[i] }

// String returns a human-readable representation of a TCF.
func (t *TCF) String() string {
	return fmt.Sprintf("TCF{id=%d duration=%d dir=%d intensity=%.4f}",
		t.TechniqueID, t.Params.Duration, t.Params.Direction, FixedToFloat(t.Params.Intensity))
}

// SynthesizeWithEngine generates audio using a motion engine, forcing the given
// technique ID. The engine's state is mutated (for stateful encode/decode consistency).
func (t *TCF) SynthesizeWithEngine(source []float64, engine *motion.Engine, nibble byte) []float64 {
	return engine.SynthesizeEventWithTechnique(source, nibble, int(t.TechniqueID), false)
}

// GestureSamples is the fixed duration of a single gesture in samples.
const GestureSamples = 11025

// FrameRate is segmentation frame size.
const FrameRate = GestureSamples

// Frame represents a fixed-size frame of audio used during frame-locked extraction.
type Frame struct {
	StartSample int64
	Data        []float64
}

// SplitFrames divides audio into fixed-size frames (no adaptive segmentation).
func SplitFrames(data []float64) []Frame {
	n := len(data)
	numFrames := (n + FrameRate - 1) / FrameRate
	frames := make([]Frame, 0, numFrames)
	for i := 0; i < n; i += FrameRate {
		end := i + FrameRate
		if end > n {
			end = n
		}
		frameData := make([]float64, end-i)
		copy(frameData, data[i:end])
		frames = append(frames, Frame{
			StartSample: int64(i),
			Data:        frameData,
		})
	}
	return frames
}