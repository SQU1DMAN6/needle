package dictionary

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"sort"

	"needle/internal/tcf"
)

// Dictionary is a locked, immutable set of canonical technique entries.
// It maps technique_id to a canonical TCF entry.
// Dictionary MUST NOT be re-derived during encode/decode cycles.
type Dictionary struct {
	Entries      []Entry         `json:"entries"`
	LockHash     [32]byte        `json:"lock_hash"`
	Threshold    float64         `json:"threshold"`
	ByteToTCFMap []tcf.TCF       `json:"byte_to_tcf"` // 16 entries mapping nibble → canonical TCF
}

// Entry is a single technique entry with frequency count.
type Entry struct {
	TechniqueID uint16  `json:"technique_id"`
	Count       int     `json:"count"`
	Frequency   float64 `json:"frequency"`
	CanonicalTCF tcf.TCF `json:"canonical_tcf"`
}

// BuildDictionary constructs a dictionary from a raw gesture log.
// threshold λ: only techniques with frequency >= threshold are included.
func BuildDictionary(records []RawGestureRecord, threshold float64) *Dictionary {
	if threshold <= 0 {
		threshold = 0.01 // default λ
	}

	// Count technique_id occurrences
	freqMap := make(map[uint16]int)
	for _, r := range records {
		freqMap[r.TechniqueID]++
	}

	total := len(records)
	var entries []Entry

	for tid, count := range freqMap {
		freq := float64(count) / float64(total)
		if freq < threshold {
			continue
		}

		// Build canonical TCF from the first occurrence of this technique
		canonicalTCF := buildCanonicalTCF(tid, records, freqMap)

		entries = append(entries, Entry{
			TechniqueID:  tid,
			Count:        count,
			Frequency:    freq,
			CanonicalTCF: canonicalTCF,
		})
	}

	// Sort by technique_id for deterministic ordering
	sort.Slice(entries, func(i, j int) bool {
		return entries[i].TechniqueID < entries[j].TechniqueID
	})

	d := &Dictionary{
		Entries:   entries,
		Threshold: threshold,
	}

	// Build byte→TCF map: map the top 16 most frequent techniques to nibble values 0-15
	d.buildByteToTCFMap()

	// Compute lock hash
	d.computeLockHash()

	return d
}

// buildCanonicalTCF creates a single canonical TCF for a technique_id
// by aggregating parameters from all occurrences.
func buildCanonicalTCF(tid uint16, records []RawGestureRecord, freqMap map[uint16]int) tcf.TCF {
	var totalDuration int64
	var totalIntensity float64
	var count int

	for _, r := range records {
		if r.TechniqueID == tid {
			totalDuration += r.Duration
			totalIntensity += r.Intensity
			count++
		}
	}

	avgDuration := int64(11025) // default ~250ms
	avgIntensity := 0.6
	if count > 0 {
		avgDuration = totalDuration / int64(count)
		avgIntensity = totalIntensity / float64(count)
	}

	return tcf.TCF{
		TechniqueID: tid,
		Params: tcf.TCParams{
			Duration:        avgDuration,
			FaderCurve:      tcf.FaderSmooth,
			PlatterVelocity: tcf.FloatToFixed(0.8),
			Direction:       tcf.DirForward,
			Intensity:       tcf.FloatToFixed(avgIntensity),
		},
	}
}

// buildByteToTCFMap creates the deterministic byte→TCF mapping.
// The top 16 techniques (by frequency) map to nibble values 0-15.
func (d *Dictionary) buildByteToTCFMap() {
	d.ByteToTCFMap = make([]tcf.TCF, 16)

	// Sort entries by frequency descending
	sorted := make([]Entry, len(d.Entries))
	copy(sorted, d.Entries)
	sort.Slice(sorted, func(i, j int) bool {
		return sorted[i].Frequency > sorted[j].Frequency
	})

	for i := 0; i < 16 && i < len(sorted); i++ {
		d.ByteToTCFMap[i] = sorted[i].CanonicalTCF
	}

	// Fill remaining slots with technique 0 if needed
	if len(sorted) < 16 && len(d.Entries) > 0 {
		fill := d.Entries[0].CanonicalTCF
		for i := len(sorted); i < 16; i++ {
			d.ByteToTCFMap[i] = fill
		}
	}
}

// computeLockHash computes SHA256 of the sorted dictionary entries.
func (d *Dictionary) computeLockHash() {
	h := sha256.New()
	for _, e := range d.Entries {
		h.Write([]byte(fmt.Sprintf("%d:%d:%d",
			e.TechniqueID,
			e.CanonicalTCF.Params.Duration,
			e.CanonicalTCF.Params.Intensity,
		)))
	}
	copy(d.LockHash[:], h.Sum(nil))
}

// LookupByte returns the canonical TCF for a nibble (0-15).
// This is the deterministic mapping used during encode/decode.
func (d *Dictionary) LookupByte(nibble byte) *tcf.TCF {
	if int(nibble) >= len(d.ByteToTCFMap) {
		return &d.ByteToTCFMap[0]
	}
	return &d.ByteToTCFMap[nibble]
}

// InverseMap recovers the nibble value from a matched TCF by
// finding the closest entry in the byte→TCF map.
func (d *Dictionary) InverseMap(matchedTCF *tcf.TCF) byte {
	bestNibble := byte(0)
	bestDist := -1.0

	for i := 0; i < len(d.ByteToTCFMap); i++ {
		ref := &d.ByteToTCFMap[i]
		dist := tcfDistance(matchedTCF, ref)
		if bestDist < 0 || dist < bestDist {
			bestDist = dist
			bestNibble = byte(i)
		}
	}

	return bestNibble
}

// tcfDistance computes a simple distance between two TCFs for inverse mapping.
func tcfDistance(a, b *tcf.TCF) float64 {
	diff := float64(a.TechniqueID) - float64(b.TechniqueID)
	if diff < 0 {
		diff = -diff
	}
	// Weight: technique ID match is primary
	if diff == 0 {
		// Same technique: compare intensity
		iDiff := tcf.FixedToFloat(a.Params.Intensity) - tcf.FixedToFloat(b.Params.Intensity)
		if iDiff < 0 {
			iDiff = -iDiff
		}
		return iDiff * 0.5
	}
	return diff * 1.0
}

// SaveToFile serializes the dictionary to a JSON file.
func (d *Dictionary) SaveToFile(path string) error {
	data, err := json.MarshalIndent(d, "", "  ")
	if err != nil {
		return fmt.Errorf("cannot marshal dictionary: %w", err)
	}
	if err := os.WriteFile(path, data, 0644); err != nil {
		return fmt.Errorf("cannot write dictionary: %w", err)
	}
	return nil
}

// LoadFromFile deserializes a dictionary from a JSON file.
func LoadFromFile(path string) (*Dictionary, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("cannot read dictionary: %w", err)
	}
	var d Dictionary
	if err := json.Unmarshal(data, &d); err != nil {
		return nil, fmt.Errorf("cannot unmarshal dictionary: %w", err)
	}
	return &d, nil
}

// HashString returns the hex-encoded lock hash.
func (d *Dictionary) HashString() string {
	return fmt.Sprintf("%x", d.LockHash[:8])
}

// RawGestureRecord is a raw extracted gesture before canonicalization.
type RawGestureRecord struct {
	SampleOffset int64
	TechniqueID  uint16
	Duration     int64
	Intensity    float64
	Direction    int
}

// TechniqueIDs returns the sorted list of technique IDs in the dictionary.
func (d *Dictionary) TechniqueIDs() []int {
	tids := make([]int, len(d.Entries))
	for i, e := range d.Entries {
		tids[i] = int(e.TechniqueID)
	}
	sort.Ints(tids)
	return tids
}

// FindNibbleForTechnique returns the nibble value that maps to the given technique ID.
// Returns 0 if not found.
func (d *Dictionary) FindNibbleForTechnique(tID int) byte {
	for nibble, tcf := range d.ByteToTCFMap {
		if int(tcf.TechniqueID) == tID {
			return byte(nibble)
		}
	}
	return 0
}
