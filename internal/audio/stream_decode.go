package audio

import (
	"fmt"
)

// StreamingDecoder provides memory-efficient decoding by reading cipher in segments
type StreamingDecoder struct {
	reader      *StreamingWAVReader
	baseLen     int
	cipherBuf   []float64 // Working buffer for current segment
	fileSize    int64
}

// NewStreamingDecoder creates a streaming decoder for dictionary-based TCF decode
func NewStreamingDecoder(keyBuf []float64, cipherPath string, dict interface{}, verbosity int) (*StreamingDecoder, error) {
	reader, err := NewStreamingWAVReader(cipherPath)
	if err != nil {
		return nil, err
	}

	fileInfo, err := reader.File.Stat()
	if err != nil {
		reader.Close()
		return nil, err
	}

	baseLen := int(0.22 * float64(SampleRate))

	return &StreamingDecoder{
		reader:    reader,
		baseLen:   baseLen,
		cipherBuf: make([]float64, baseLen*2), // Buffer up to 2 gestures
		fileSize:  fileInfo.Size(),
	}, nil
}

// ReadSegment reads a segment starting at the given position
func (d *StreamingDecoder) ReadSegment(pos int64, segLen int) ([]float64, error) {
	buf := make([]float64, segLen)
	
	// Seek to position
	_, err := d.reader.File.Seek(d.reader.DataStart+pos*2, 0)
	if err != nil {
		return nil, fmt.Errorf("cannot seek: %w", err)
	}

	// Read raw bytes
	byteCount := segLen * 2
	byteBuf := make([]byte, byteCount)
	n, err := d.reader.File.Read(byteBuf)
	if err != nil {
		return nil, err
	}

	// Convert to float64
	samplesRead := n / 2
	for i := 0; i < samplesRead; i++ {
		low := byteBuf[i*2]
		high := byteBuf[i*2+1]
		sample := float64(int16(uint16(high)<<8 | uint16(low)))
		buf[i] = sample / 32768.0
	}

	return buf[:samplesRead], nil
}

// EstimatedGestures returns estimated number of gestures for progress tracking
func (d *StreamingDecoder) EstimatedGestures() int64 {
	estimatedSamples := (d.fileSize - 44) / 2
	return estimatedSamples / int64(d.baseLen)
}

// Close closes the decoder
func (d *StreamingDecoder) Close() error {
	return d.reader.Close()
}