package audio

import (
	"encoding/binary"
	"fmt"
	"os"
)

// StreamingWAVReader reads WAV files in a streaming fashion to reduce memory usage
type StreamingWAVReader struct {
	File         *os.File
	DataStart    int64
	DataSize     int64
	samplesRead  int64
}

// NewStreamingWAVReader creates a streaming WAV reader
func NewStreamingWAVReader(path string) (*StreamingWAVReader, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("cannot open file: %w", err)
	}

	// Read WAV header (44 bytes standard)
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

	// Extract data chunk size from header (at offset 40)
	dataSize := int64(binary.LittleEndian.Uint32(header[40:44]))
	
	// Data starts after 44-byte header
	dataStart := int64(44)

	reader := &StreamingWAVReader{
		File:      f,
		DataStart: dataStart,
		DataSize:  dataSize,
	}

	return reader, nil
}

// ReadSamples reads up to n samples into the provided float64 buffer
// Returns the actual number of samples read
func (r *StreamingWAVReader) ReadSamples(buf []float64) (int, error) {
	if r.SamplesRead() >= r.DataSize && r.DataSize > 0 {
		return 0, fmt.Errorf("end of WAV data")
	}

	// Calculate how many samples we can read
	remaining := r.DataSize - r.SamplesRead()
	if r.DataSize == 0 {
		remaining = int64(len(buf) * 2) // Unknown size, try to read
	} else {
		if remaining > int64(len(buf)*2) {
			remaining = int64(len(buf) * 2)
		}
	}

	// Read raw bytes (2 bytes per 16-bit sample)
	byteCount := int(remaining)
	byteBuf := make([]byte, byteCount)
	n, err := r.File.Read(byteBuf)
	if err != nil {
		return 0, err
	}

	// Convert to float64 samples
	samplesRead := n / 2
	for i := 0; i < samplesRead; i++ {
		low := byteBuf[i*2]
		high := byteBuf[i*2+1]
		// Signed 16-bit little-endian to float64
		sample := float64(int16(uint16(high)<<8 | uint16(low)))
		buf[i] = sample / 32768.0 // Normalize to [-1, 1]
	}

	r.SetSamplesRead(r.SamplesRead() + int64(samplesRead))
	return samplesRead, nil
}

// ReadSamplesAtPosition reads samples from a specific position in the file
func (r *StreamingWAVReader) ReadSamplesAtPosition(buf []float64, position int64) (int, error) {
	if position >= r.DataSize && r.DataSize > 0 {
		return 0, fmt.Errorf("end of WAV data")
	}

	// Seek to position
	if _, err := r.File.Seek(r.DataStart+position*2, 0); err != nil {
		return 0, fmt.Errorf("cannot seek: %w", err)
	}

	// Read samples
	samplesNeeded := len(buf)
	if r.DataSize > 0 {
		remaining := r.DataSize - position
		if remaining < int64(samplesNeeded*2) {
			samplesNeeded = int(remaining / 2)
		}
	}

	byteCount := samplesNeeded * 2
	byteBuf := make([]byte, byteCount)
	n, err := r.File.Read(byteBuf)
	if err != nil {
		return 0, err
	}

	// Convert to float64
	actualSamples := n / 2
	for i := 0; i < actualSamples; i++ {
		low := byteBuf[i*2]
		high := byteBuf[i*2+1]
		sample := float64(int16(uint16(high)<<8 | uint16(low)))
		buf[i] = sample / 32768.0
	}

	return actualSamples, nil
}

// SamplesRead returns the number of samples read so far
func (r *StreamingWAVReader) SamplesRead() int64 {
	return r.samplesRead
}

// SetSamplesRead sets the number of samples read
func (r *StreamingWAVReader) SetSamplesRead(val int64) {
	r.samplesRead = val
}

// Close closes the WAV reader
func (r *StreamingWAVReader) Close() error {
	return r.File.Close()
}

// SkipSamples skips ahead by the specified number of samples
func (r *StreamingWAVReader) SkipSamples(count int64) error {
	bytesToSkip := count * 2
	if r.DataSize > 0 {
		remaining := r.DataSize - r.SamplesRead()
		if bytesToSkip > remaining {
			bytesToSkip = remaining
		}
	}

	_, err := r.File.Seek(bytesToSkip, 1) // Seek relative to current position
	if err != nil {
		return err
	}
	r.SetSamplesRead(r.SamplesRead() + bytesToSkip/2)
	return nil
}
