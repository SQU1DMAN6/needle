package audio

import (
	"encoding/binary"
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

var sampleRateBytes = func() []byte {
	sr := uint32(SampleRate)
	return []byte{
		byte(sr),
		byte(sr >> 8),
		byte(sr >> 16),
		byte(sr >> 24),
	}
}()

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

// WriteSamples writes 16-bit PCM samples to a WAV file
func WriteSamples(out *os.File, data []int) error {
	// Write little-endian 16-bit samples
	buf := make([]byte, len(data)*2)
	for i, v := range data {
		buf[i*2] = byte(v & 0xFF)
		buf[i*2+1] = byte((v >> 8) & 0xFF)
	}
	_, err := out.Write(buf)
	return err
}

// UpdateWAVHeader seeks back and writes the correct WAV header with actual data size
func UpdateWAVHeader(out *os.File, samplesWritten int64) error {
	dataSize := samplesWritten * 2 // 2 bytes per sample
	fileSize := dataSize + 36

	if _, err := out.Seek(0, 0); err != nil {
		return err
	}

	// Write RIFF header
	if _, err := out.Write([]byte("RIFF")); err != nil {
		return err
	}
	if _, err := out.Write(intToBytes(int64(fileSize))); err != nil {
		return err
	}
	if _, err := out.Write([]byte("WAVE")); err != nil {
		return err
	}

	// Write fmt chunk
	if _, err := out.Write([]byte("fmt ")); err != nil {
		return err
	}
	if _, err := out.Write([]byte{16, 0, 0, 0}); err != nil {
		return err
	}
	if _, err := out.Write([]byte{1, 0}); err != nil {
		return err
	}
	if _, err := out.Write([]byte{1, 0}); err != nil {
		return err
	}
	if _, err := out.Write(sampleRateBytes); err != nil {
		return err
	}
	byteRate := int64(SampleRate) * 2
	if _, err := out.Write(intToBytes(byteRate)); err != nil {
		return err
	}
	if _, err := out.Write([]byte{2, 0}); err != nil {
		return err
	}
	if _, err := out.Write([]byte{16, 0}); err != nil {
		return err
	}

	// Write data chunk header
	if _, err := out.Write([]byte("data")); err != nil {
		return err
	}
	if _, err := out.Write(intToBytes(dataSize)); err != nil {
		return err
	}

	return nil
}

// intToBytes converts an int64 to 4 little-endian bytes
func intToBytes(val int64) []byte {
	return []byte{
		byte(val),
		byte(val >> 8),
		byte(val >> 16),
		byte(val >> 24),
	}
}

// WAVReader provides streaming WAV reading for large files
type WAVReader struct {
	file      *os.File
	dataStart int64
	dataSize  int64
	samplesRead int64
}

// NewWAVReader creates a streaming WAV reader
func NewWAVReader(path string) (*WAVReader, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("cannot open file: %w", err)
	}

	// Read WAV header
	header := make([]byte, 44)
	if _, err := f.Read(header); err != nil {
		f.Close()
		return nil, fmt.Errorf("cannot read WAV header: %w", err)
	}

	// Verify RIFF header
	if string(header[0:4]) != "RIFF" {
		f.Close()
		return nil, fmt.Errorf("invalid WAV file: missing RIFF header")
	}

	// Find data chunk
	// Skip to data chunk (typically at offset 36)
	dataStart := int64(36)
	
	reader := &WAVReader{
		file:      f,
		dataStart: dataStart,
		dataSize:  0,
	}

	// Try to read data chunk size
	if len(header) >= 40 {
		dataSize := int64(binary.LittleEndian.Uint32(header[36:40]))
		if dataSize > 0 {
			reader.dataSize = dataSize
		}
	}

	// Seek to data start
	if _, err := f.Seek(dataStart, 0); err != nil {
		f.Close()
		return nil, fmt.Errorf("cannot seek to data chunk: %w", err)
	}

	return reader, nil
}

// ReadSamples reads up to n samples into the provided buffer
func (r *WAVReader) ReadSamples(buf []int) (int, error) {
	if r.samplesRead >= r.dataSize && r.dataSize > 0 {
		return 0, fmt.Errorf("end of WAV data")
	}

	// Calculate how many samples we can read
	remaining := r.dataSize - r.samplesRead
	if r.dataSize == 0 {
		remaining = int64(len(buf) * 2) // Unknown size, try to read
	} else {
		if remaining > int64(len(buf)*2) {
			remaining = int64(len(buf) * 2)
		}
	}

	// Read raw bytes
	byteBuf := make([]byte, remaining)
	n, err := r.file.Read(byteBuf)
	if err != nil {
		return 0, err
	}

	// Convert to int samples
	samplesRead := n / 2
	for i := 0; i < samplesRead; i++ {
		low := byteBuf[i*2]
		high := byteBuf[i*2+1]
		// Signed 16-bit little-endian
		sample := int(int16(uint16(high)<<8 | uint16(low)))
		buf[i] = sample
	}

	r.samplesRead += int64(samplesRead)
	return samplesRead, nil
}

// Close closes the WAV reader
func (r *WAVReader) Close() error {
	return r.file.Close()
}

// ReadChunks reads the WAV file in chunks for processing large files
func ReadChunks(path string, chunkSize int, processFunc func([]float64) error) error {
	f, err := os.Open(path)
	if err != nil {
		return fmt.Errorf("cannot open file: %w", err)
	}
	defer f.Close()

	decoder := wav.NewDecoder(f)
	buf, err := decoder.FullPCMBuffer()
	if err != nil {
		return fmt.Errorf("cannot decode WAV: %w", err)
	}

	if buf.Format.NumChannels != Channels {
		return fmt.Errorf("expected mono audio, got %d channels", buf.Format.NumChannels)
	}

	if buf.Format.SampleRate != SampleRate {
		return fmt.Errorf("expected %d Hz, got %d Hz", SampleRate, buf.Format.SampleRate)
	}

	floatBuf := buf.AsFloat32Buffer()
	data := floatBuf.Data

	// Process in chunks
	for i := 0; i < len(data); i += chunkSize {
		end := i + chunkSize
		if end > len(data) {
			end = len(data)
		}
		
		chunk := make([]float64, end-i)
		for j := i; j < end; j++ {
			chunk[j-i] = float64(data[j])
		}

		if err := processFunc(chunk); err != nil {
			return err
		}
	}

	return nil
}