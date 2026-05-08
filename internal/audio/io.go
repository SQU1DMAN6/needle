package audio

import (
	"fmt"
	"math"
	"os"

	"github.com/go-audio/audio"
	"github.com/go-audio/wav"
)

const (
	SampleRate = 44100
	MinLength  = SampleRate
	Channels   = 1
	BitDepth   = 16
)

// LoadWAV loads a WAV file and returns float64 samples
func LoadWAV(path string) ([]float64, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("cannot open file: %w", err)
	}
	defer f.Close()

	decoder := wav.NewDecoder(f)
	buf, err := decoder.FullPCMBuffer()
	if err != nil {
		return nil, fmt.Errorf("cannot decode WAV: %w", err)
	}

	if buf.Format.NumChannels != Channels {
		return nil, fmt.Errorf("expected mono audio, got %d channels", buf.Format.NumChannels)
	}

	if buf.Format.SampleRate != SampleRate {
		return nil, fmt.Errorf("expected %d Hz, got %d Hz", SampleRate, buf.Format.SampleRate)
	}

	floatBuf := buf.AsFloat32Buffer()
	data := make([]float64, len(floatBuf.Data))
	for i, v := range floatBuf.Data {
		data[i] = float64(v)
	}
	return data, nil
}

// SaveWAV saves float64 samples to a WAV file
func SaveWAV(path string, data []float64) error {
	out, err := os.Create(path)
	if err != nil {
		return fmt.Errorf("cannot create file: %w", err)
	}
	defer out.Close()

	intData := make([]int, len(data))
	for i, v := range data {
		sample := int(math.Round(v * 32767.0))
		if sample > 32767 {
			sample = 32767
		}
		if sample < -32768 {
			sample = -32768
		}
		intData[i] = sample
	}

	encoder := wav.NewEncoder(out, SampleRate, BitDepth, Channels, 1)
	buf := &audio.IntBuffer{
		Data:   intData,
		Format: &audio.Format{SampleRate: SampleRate, NumChannels: Channels},
	}
	if err := encoder.Write(buf); err != nil {
		return fmt.Errorf("cannot write audio: %w", err)
	}
	return encoder.Close()
}

// SampleAt performs linear interpolation on audio data
func SampleAt(source []float64, position float64) float64 {
	i := int(math.Floor(position))
	frac := position - float64(i)
	n := len(source)
	idx0 := ((i % n) + n) % n
	idx1 := (idx0 + 1) % n
	return source[idx0]*(1-frac) + source[idx1]*frac
}

// WrapPosition handles circular buffer wraparound
func WrapPosition(pos float64, n int) float64 {
	if pos < 0 {
		pos = float64(n) + math.Mod(pos, float64(n))
	}
	return math.Mod(pos, float64(n))
}
