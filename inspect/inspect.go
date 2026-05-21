package inspect

import (
	"fmt"
	"os"
	"sort"

	"needle/internal/audio"
	"needle/internal/decode"
	"needle/internal/motion"
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
