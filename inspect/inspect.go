package inspect

import (
	"fmt"
	"math"
	"os"

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
		segment := engine.SynthesizeEvent(keyBuf, nibble)
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

func InspectCipher(keyBuf, cipherBuf []float64) ([]GestureRecord, error) {
	if len(keyBuf) < audio.MinLength {
		return nil, fmt.Errorf("key sample must be at least 1 second long")
	}

	baseLen := int(0.22 * float64(audio.SampleRate))
	engine := motion.NewEngine(keyBuf, baseLen)
	pos := 0
	records := make([]GestureRecord, 0, 256)

	for pos < len(cipherBuf) {
		bestCost := math.Inf(1)
		var bestRec GestureRecord
		var bestEngine *motion.Engine

		for n := 0; n < 16; n++ {
			candidate := engine.Clone()
			segment := candidate.SynthesizeEvent(keyBuf, byte(n))
			length := len(segment)
			if pos+length > len(cipherBuf) {
				continue
			}

			target := cipherBuf[pos : pos+length]
			cost := decode.DistanceRaw(segment, target)
			if cost < bestCost {
				bestCost = cost
				bestRec = GestureRecord{
					Index:           len(records),
					Nibble:          byte(n),
					GestureType:     candidate.LastGesture,
					SegmentLen:      length,
					Cost:            cost,
					Intensity:       candidate.LastIntensity,
					Duration:        float64(length) / float64(audio.SampleRate),
					PlatterVelocity: candidate.Physics.PlatterVelocity,
					StylusDrag:      candidate.Physics.StylusDrag,
					Crossfader:      candidate.CrossfaderPos,
				}
				bestEngine = candidate
			}
		}

		if bestEngine == nil {
			return nil, fmt.Errorf("failed to inspect cipher at position %d", pos)
		}

		records = append(records, bestRec)
		engine = bestEngine
		pos += bestRec.SegmentLen
	}

	return records, nil
}
