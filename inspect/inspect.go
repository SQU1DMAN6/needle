package inspect

import (
	"fmt"
	"os"
	"sort"

	"needle/internal/audio"
	"needle/internal/decode"
	"needle/internal/dictionary"
	"needle/internal/gesture"
	"needle/internal/motion"
	"needle/internal/tcf"
)

type GestureRecord struct {
	Index           int
	Nibble          byte
	GestureType     int
	SegmentLen      int
	Cost            float64
	Intensity       float64
	Duration        float64
	PlatterVelocity float64
	StylusDrag      float64
	Crossfader      float64
}

func LoadWAV(path string) ([]float64, error) {
	return audio.LoadWAV(path)
}

func SaveWAV(path string, data []float64) error {
	return audio.SaveWAV(path, data)
}

func FormatGestureRecord(r GestureRecord) string {
	return fmt.Sprintf("%04d nibbles=%02x gesture=%d length=%d duration=%.3fs cost=%.6f intensity=%.3f platter=%.4f stylus=%.4f crossfader=%.3f",
		r.Index, r.Nibble, r.GestureType, r.SegmentLen, r.Duration, r.Cost, r.Intensity, r.PlatterVelocity, r.StylusDrag, r.Crossfader)
}

func WriteGestureLog(path string, records []GestureRecord) error {
	f, err := os.Create(path)
	if err != nil {
		return fmt.Errorf("cannot create log file: %w", err)
	}
	defer f.Close()

	if _, err := fmt.Fprintln(f, "index nibble gesture length duration cost intensity platter stylus crossfader"); err != nil {
		return err
	}
	for _, rec := range records {
		if _, err := fmt.Fprintln(f, FormatGestureRecord(rec)); err != nil {
			return err
		}
	}
	return nil
}

// EncodePlaintext uses the legacy motion engine (stateful, PRNG-based).
func EncodePlaintext(keyBuf []float64, plain []byte) ([]float64, []GestureRecord, error) {
	if len(keyBuf) < audio.MinLength {
		return nil, nil, fmt.Errorf("key sample must be at least 1 second long")
	}

	baseLen := int(0.22 * float64(audio.SampleRate))
	engine := motion.NewEngine(keyBuf, baseLen)
	nibbles := make([]byte, 0, len(plain)*2)
	for _, b := range plain {
		nibbles = append(nibbles, b>>4, b&0x0f)
	}

	output := make([]float64, 0, len(nibbles)*baseLen)
	records := make([]GestureRecord, 0, len(nibbles))

	for i, nibble := range nibbles {
		segment := engine.SynthesizeEvent(keyBuf, nibble, i == len(nibbles)-1)
		output = append(output, segment...)
		records = append(records, GestureRecord{
			Index:           i,
			Nibble:          nibble,
			GestureType:     engine.LastGesture,
			SegmentLen:      len(segment),
			Cost:            0.0,
			Intensity:       engine.LastIntensity,
			Duration:        float64(len(segment)) / float64(audio.SampleRate),
			PlatterVelocity: engine.Physics.PlatterVelocity,
			StylusDrag:      engine.Physics.StylusDrag,
			Crossfader:      engine.CrossfaderPos,
		})
	}

	return output, records, nil
}

// EncodeLocked uses a locked dictionary for deterministic encoding.
// Uses a fresh motion engine per gesture for stateless synthesis.
// This ensures decode can reproduce identical audio for each technique ID.
func EncodeLocked(keyBuf []float64, plain []byte, dict *dictionary.Dictionary) ([]float64, []GestureRecord, error) {
	if len(keyBuf) < audio.MinLength {
		return nil, nil, fmt.Errorf("key sample must be at least 1 second long")
	}

	nibbles := make([]byte, 0, len(plain)*2)
	for _, b := range plain {
		nibbles = append(nibbles, b>>4, b&0x0f)
	}

	baseLen := int(0.22 * float64(audio.SampleRate))
	output := make([]float64, 0, len(nibbles)*baseLen)
	records := make([]GestureRecord, 0, len(nibbles))

	for i, nibble := range nibbles {
		canonicalTCF := dict.LookupByte(nibble)
		techniqueID := int(canonicalTCF.TechniqueID)

		// Fresh engine per gesture — stateless, deterministic
		engine := motion.NewEngine(keyBuf, baseLen)
		segment := engine.SynthesizeEventWithTechnique(keyBuf, nibble, techniqueID, i == len(nibbles)-1)

		output = append(output, segment...)
		records = append(records, GestureRecord{
			Index:           i,
			Nibble:          nibble,
			GestureType:     techniqueID,
			SegmentLen:      len(segment),
			Cost:            0.0,
			Intensity:       engine.LastIntensity,
			Duration:        float64(len(segment)) / float64(audio.SampleRate),
			PlatterVelocity: engine.Physics.PlatterVelocity,
			StylusDrag:      engine.Physics.StylusDrag,
			Crossfader:      engine.CrossfaderPos,
		})
	}

	return output, records, nil
}

type inspectCandidate struct {
	engine  *motion.Engine
	pos     int
	cost    float64
	records []GestureRecord
}

func InspectCipher(keyBuf, cipherBuf []float64, beamWidth int) ([]GestureRecord, error) {
	if len(keyBuf) < audio.MinLength {
		return nil, fmt.Errorf("key sample must be at least 1 second long")
	}
	if beamWidth < 1 {
		beamWidth = 1
	}

	baseLen := int(0.22 * float64(audio.SampleRate))
	initialEngine := motion.NewEngine(keyBuf, baseLen)
	beam := []inspectCandidate{{engine: initialEngine, pos: 0, cost: 0, records: nil}}
	targetLen := len(cipherBuf)
	best := beam[0]

	for step := 0; step < 2048 && len(beam) > 0; step++ {
		nextBeam := make([]inspectCandidate, 0, len(beam)*16)
		for _, cand := range beam {
			if cand.pos == targetLen {
				if cand.pos > best.pos || (cand.pos == best.pos && cand.cost < best.cost) {
					best = cand
				}
				continue
			}

			for n := 0; n < 16; n++ {
				finalEngine := cand.engine.Clone()
				finalSegment := finalEngine.SynthesizeEvent(keyBuf, byte(n), false)
				finalPos := cand.pos + len(finalSegment)
				final := finalPos == targetLen
				if final {
					finalEngine = cand.engine.Clone()
					finalSegment = finalEngine.SynthesizeEvent(keyBuf, byte(n), true)
					finalPos = cand.pos + len(finalSegment)
				}
				if finalPos > targetLen {
					continue
				}
				segment := finalSegment
				length := len(segment)
				nextPos := finalPos

				target := cipherBuf[cand.pos:nextPos]
				cost := decode.DistanceRaw(segment, target)
				records := append(append([]GestureRecord(nil), cand.records...), GestureRecord{
					Index:           len(cand.records),
					Nibble:          byte(n),
					GestureType:     finalEngine.LastGesture,
					SegmentLen:      length,
					Cost:            cost,
					Intensity:       finalEngine.LastIntensity,
					Duration:        float64(length) / float64(audio.SampleRate),
					PlatterVelocity: finalEngine.Physics.PlatterVelocity,
					StylusDrag:      finalEngine.Physics.StylusDrag,
					Crossfader:      finalEngine.CrossfaderPos,
				})
				nextBeam = append(nextBeam, inspectCandidate{
					engine:  finalEngine,
					pos:     nextPos,
					cost:    cand.cost + cost,
					records: records,
				})
			}
		}

		if len(nextBeam) == 0 {
			break
		}

		beam = pruneInspectCandidates(nextBeam, beamWidth)
		for _, cand := range beam {
			if cand.pos > best.pos || (cand.pos == best.pos && cand.cost < best.cost) {
				best = cand
			}
		}
		if best.pos == targetLen {
			return best.records, nil
		}
	}

	if best.pos == 0 {
		return nil, fmt.Errorf("failed to inspect cipher: no viable gesture path found")
	}
	return best.records, nil
}

func pruneInspectCandidates(candidates []inspectCandidate, limit int) []inspectCandidate {
	sort.Slice(candidates, func(i, j int) bool {
		ti := candidates[i]
		jj := candidates[j]
		scoreI := ti.cost / float64(ti.pos+1)
		scoreJ := jj.cost / float64(jj.pos+1)
		if scoreI == scoreJ {
			return ti.pos > jj.pos
		}
		return scoreI < scoreJ
	})
	if len(candidates) <= limit {
		return candidates
	}
	return candidates[:limit]
}

// ============================================================
// Frame-Locked Extraction
// ============================================================

// ExtractRawGestures performs frame-locked gesture extraction from audio.
// Uses fixed 11025-sample frames (~250ms) to match gesture duration.
// Returns a list of RawGestureRecord suitable for dictionary construction.
func ExtractRawGestures(keyBuf, sampleBuf []float64) ([]dictionary.RawGestureRecord, error) {
	if len(keyBuf) < audio.MinLength {
		return nil, fmt.Errorf("key sample must be at least 1 second long")
	}

	frames := tcf.SplitFrames(sampleBuf)
	records := make([]dictionary.RawGestureRecord, 0, len(frames))
	baseLen := int(0.22 * float64(audio.SampleRate))

	for _, frame := range frames {
		bestID := uint16(0)
		bestCost := -1.0

		for tid := 0; tid < 32; tid++ {
			tmpl := gesture.Templates[tid]
			if tmpl == nil {
				continue
			}

			// Synthesize a reference using the motion engine with this technique
			refEngine := motion.NewEngine(keyBuf, baseLen)
			refSegment := refEngine.SynthesizeEventWithTechnique(keyBuf, 0, tid, false)

			// Ensure length matches by truncating or padding
			if len(refSegment) > len(frame.Data) {
				refSegment = refSegment[:len(frame.Data)]
			} else if len(refSegment) < len(frame.Data) {
				// Pad with zeros
				padded := make([]float64, len(frame.Data))
				copy(padded, refSegment)
				refSegment = padded
			}

			cost := decode.DistanceRaw(refSegment, frame.Data)
			if bestCost < 0 || cost < bestCost {
				bestCost = cost
				bestID = uint16(tid)
			}
		}

		records = append(records, dictionary.RawGestureRecord{
			SampleOffset: frame.StartSample,
			TechniqueID:  bestID,
			Duration:     int64(len(frame.Data)),
			Intensity:    0.6,
			Direction:    0,
		})
	}

	return records, nil
}

// ============================================================
// DecodeLocked — decodes cipher audio using a locked dictionary.
// Uses motion engine for reference synthesis (realistic audio).
// ============================================================

// DecodeLocked decodes ciphertext audio using a locked dictionary.
// Uses fresh motion engines per gesture for matching — identical to EncodeLocked.
// progressFn is an optional callback called with (current, total) after each gesture.
func DecodeLocked(keyBuf, cipherBuf []float64, dict *dictionary.Dictionary, progressFn func(current, total int)) ([]byte, error) {
	if len(keyBuf) < audio.MinLength {
		return nil, fmt.Errorf("key sample must be at least 1 second long")
	}

	baseLen := int(0.22 * float64(audio.SampleRate))
	cipherLen := len(cipherBuf)
	matchedNibbles := make([]byte, 0)
	pos := 0
	gestureCount := 0
	totalEst := cipherLen / baseLen
	if totalEst < 1 {
		totalEst = 1
	}

	for pos < cipherLen {
		gestureCount++
		bestNibble := byte(0)
		bestCost := -1.0

		for n := 0; n < 16; n++ {
			refTCF := dict.LookupByte(byte(n))
			techniqueID := int(refTCF.TechniqueID)

			// Fresh engine per candidate — same as EncodeLocked
			refEngine := motion.NewEngine(keyBuf, baseLen)
			refSegment := refEngine.SynthesizeEventWithTechnique(keyBuf, byte(n), techniqueID, false)

			// Compare against the cipher at current position
			endPos := pos + len(refSegment)
			if endPos > cipherLen {
				endPos = cipherLen
			}
			target := cipherBuf[pos:endPos]

			cost := decode.DistanceRaw(refSegment[:len(target)], target)
			if bestCost < 0 || cost < bestCost {
				bestCost = cost
				bestNibble = byte(n)
			}
		}

		matchedNibbles = append(matchedNibbles, bestNibble)

		// Advance by the length of the best-matching reference
		refTCF := dict.LookupByte(bestNibble)
		refEngine := motion.NewEngine(keyBuf, baseLen)
		refSegment := refEngine.SynthesizeEventWithTechnique(keyBuf, bestNibble, int(refTCF.TechniqueID), false)
		pos += len(refSegment)

		// Report progress
		if progressFn != nil {
			progressFn(gestureCount, totalEst)
		}
	}

	// Convert nibbles to bytes
	decoded := make([]byte, (len(matchedNibbles)+1)/2)
	for i := 0; i < len(matchedNibbles); i += 2 {
		high := matchedNibbles[i] & 0x0f
		low := byte(0)
		if i+1 < len(matchedNibbles) {
			low = matchedNibbles[i+1] & 0x0f
		}
		decoded[i/2] = (high << 4) | low
	}

	return decoded, nil
}

// ValidateCipher checks that observed gesture log matches expected log.
// Fails hard on any mismatch (spec §8).
func ValidateCipher(observed, expected []GestureRecord) error {
	if len(observed) != len(expected) {
		return fmt.Errorf("GESTURE MISMATCH: observed %d gestures, expected %d",
			len(observed), len(expected))
	}

	for i := range observed {
		if observed[i].GestureType != expected[i].GestureType ||
			observed[i].Nibble != expected[i].Nibble {
			return fmt.Errorf(
				"GESTURE MISMATCH at index %d: observed {nibble=%d gesture=%d}, expected {nibble=%d gesture=%d}",
				i, observed[i].Nibble, observed[i].GestureType,
				expected[i].Nibble, expected[i].GestureType,
			)
		}
	}

	return nil
}