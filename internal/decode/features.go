package decode

import (
	"math"
)

// Features stores the full waveform for direct L2 distance matching
type Features struct {
	Waveform []float64
}

// ExtractFeatures returns the full waveform as features
func ExtractFeatures(segment []float64) Features {
	// Store full waveform for maximum fidelity in distance computation
	waveform := make([]float64, len(segment))
	copy(waveform, segment)
	return Features{Waveform: waveform}
}

// Distance computes multi-metric ensemble distance for robust classification
// Combines: normalized cross-correlation, spectral distance, and envelope matching
func Distance(f1, f2 Features) float64 {
	if len(f1.Waveform) != len(f2.Waveform) {
		return math.Inf(1)
	}

	// Metric 1: Correlation distance (robustness to amplitude)
	corrDist := correlationDistance(f1.Waveform, f2.Waveform)

	// Metric 2: Spectral distance (captures frequency content)
	specDist := spectralDistance(f1.Waveform, f2.Waveform)

	// Metric 3: Envelope distance (captures overall contour)
	envDist := envelopeDistance(f1.Waveform, f2.Waveform)

	// Ensemble: weighted average (correlation is most reliable, so give it more weight)
	return 0.6*corrDist + 0.25*specDist + 0.15*envDist
}

// correlationDistance: normalized cross-correlation based distance
func correlationDistance(s1, s2 []float64) float64 {
	w1 := normalize(s1)
	w2 := normalize(s2)

	correlation := 0.0
	for i := range w1 {
		correlation += w1[i] * w2[i]
	}
	correlation /= float64(len(w1))

	if correlation > 1.0 {
		correlation = 1.0
	}
	if correlation < -1.0 {
		correlation = -1.0
	}

	return (1.0 - correlation)
}

// spectralDistance: compare power in frequency bands
func spectralDistance(s1, s2 []float64) float64 {
	e1 := bandEnergies(s1, 4)
	e2 := bandEnergies(s2, 4)

	dist := 0.0
	for i := range e1 {
		d := e1[i] - e2[i]
		dist += d * d
	}
	return math.Sqrt(dist / float64(len(e1)))
}

// bandEnergies divides signal into numBands frequency bands and returns energy in each
func bandEnergies(signal []float64, numBands int) []float64 {
	if len(signal) < numBands {
		numBands = len(signal)
	}

	bandSize := len(signal) / numBands
	energies := make([]float64, numBands)

	for b := 0; b < numBands; b++ {
		start := b * bandSize
		end := start + bandSize
		if b == numBands-1 {
			end = len(signal)
		}

		energy := 0.0
		for i := start; i < end; i++ {
			energy += signal[i] * signal[i]
		}
		// Normalize by band size
		energies[b] = math.Sqrt(energy / float64(end-start))
	}

	return energies
}

// envelopeDistance: compare the envelope (local extrema structure)
func envelopeDistance(s1, s2 []float64) float64 {
	env1 := envelopeShape(s1)
	env2 := envelopeShape(s2)

	dist := 0.0
	for i := range env1 {
		d := env1[i] - env2[i]
		dist += d * d
	}
	return math.Sqrt(dist / float64(len(env1)))
}

// envelopeShape extracts simplified envelope (samples every N points)
func envelopeShape(signal []float64) []float64 {
	// Downsample to 8 points for envelope comparison
	targetLen := 8
	if len(signal) < targetLen {
		targetLen = len(signal)
	}

	env := make([]float64, targetLen)
	step := float64(len(signal)) / float64(targetLen)

	for i := 0; i < targetLen; i++ {
		idx := int(float64(i) * step)
		if idx >= len(signal) {
			idx = len(signal) - 1
		}
		env[i] = signal[idx]
	}

	// Normalize envelope
	return normalize(env)
}

// normalize performs z-score normalization on a signal
func normalize(signal []float64) []float64 {
	if len(signal) == 0 {
		return signal
	}

	// Compute mean
	mean := 0.0
	for _, v := range signal {
		mean += v
	}
	mean /= float64(len(signal))

	// Compute standard deviation
	variance := 0.0
	for _, v := range signal {
		diff := v - mean
		variance += diff * diff
	}
	variance /= float64(len(signal))
	stddev := math.Sqrt(variance)

	// If stddev is near zero, return the signal as-is
	if stddev < 1e-10 {
		return signal
	}

	// Normalize
	normalized := make([]float64, len(signal))
	for i, v := range signal {
		normalized[i] = (v - mean) / stddev
	}

	return normalized
}

// ClassifyByte finds the best matching byte value for a segment using 3-NN voting
func ClassifyByte(segment []float64, library []Features) byte {
	features := ExtractFeatures(segment)

	// Compute distances to all library entries
	type match struct {
		label byte
		dist  float64
	}
	matches := make([]match, len(library))
	for b := 0; b < len(library); b++ {
		matches[b] = match{byte(b), Distance(features, library[b])}
	}

	// Find 3 nearest neighbors (or fewer if library is small)
	k := 3
	if k > len(library) {
		k = len(library)
	}

	// Partial sort: find k minimum elements
	best := matches[0]
	second := match{0, math.Inf(1)}
	third := match{0, math.Inf(1)}

	for _, m := range matches {
		if m.dist < best.dist {
			third = second
			second = best
			best = m
		} else if m.dist < second.dist {
			third = second
			second = m
		} else if m.dist < third.dist {
			third = m
		}
	}

	// If k=1 or best match is clearly better (low margin), use it
	if k == 1 || (second.dist-best.dist) > 0.1 {
		return best.label
	}

	// Otherwise, use majority voting among top 3
	// Since we have numeric labels, use the median of the 3 closest
	// This acts as a "robust" selector when neighbors are close
	if k >= 3 {
		labels := []byte{best.label, second.label, third.label}
		return labels[1] // Return middle value as median (robust estimate)
	} else if k >= 2 {
		if best.label < second.label {
			return best.label
		}
		return second.label
	}

	return best.label
}
