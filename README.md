# Needle - Key-Conditioned Audio Gesture Cipher

A proof-of-concept implementation of the Needle algorithm in Go, encoding plaintext into audio gestures synthesized from a key sample.

## Overview

Needle encodes plaintext into a sequence of parameterized audio gestures applied to a key sample, producing a cipher audio file. Decoding uses robust multi-metric ensemble distance matching to perfectly recover the original plaintext.

**Current Status**: ✓ Perfect round-trip encoding/decoding with 100% accuracy

## Requirements

- Go 1.21+
- Mono WAV files at 44100 Hz, 16-bit
- Key sample must be at least 1 second long

## Usage

### Encoding

```bash
needle encode -key sample.wav -input plaintext.txt -output cipher.wav
```

Flags:
`-key`    Path to key sample WAV file (required, ≥1 second)
`-input`  Path to plaintext file to encode (required)
`-output` Path to output cipher WAV file (required)

### Decoding

```bash
needle decode -key sample.wav -input cipher.wav -output plaintext.txt
```

`-key`    Same key sample used for encoding (required)
`-input`  Path to cipher WAV file (required)
`-output` Path to output plaintext file (required)

## How It Works

### Encoding Pipeline

1. **Plaintext → Nibbles**: Each input byte is split into two 4-bit nibbles (values 0–15)
2. **Nibble → Gesture**: Each nibble maps to a deterministic gesture through seeded PRNG
3. **Gesture Synthesis**: Gestures are synthesized as 250ms audio segments via motion engine:
   - Position and velocity curves driven by gesture type
   - Gating (gain modulation) applied per gesture
   - Segment concatenated to output cipher
4. **Cipher Output**: WAV file containing all gesture segments

### Decoding Pipeline

1. **Cipher Segments**: Parse cipher into 250ms segments
2. **Reference Library**: Generate 16 reference gesture signatures (one per nibble value) using the same key
3. **Classification**: For each cipher segment, find best-matching nibble using ensemble distance metric
4. **Nibble → Byte**: Reconstruct plaintext bytes from nibble pairs
5. **Output**: Write recovered plaintext

## Gesture Architecture

### 32 Distinct Gesture Types

The system synthesizes 32 parameterized gesture types, providing rich variation:

- Forward drags (with and without modulation)
- Reverse pulls (with decay and envelope shaping)
- Oscillatory scratches (baby scratch, stutter scratch, dual oscillation)
- Pulsing/gating effects (transformer cut, double pulse, asymmetric pulse)
- Tape effects (deceleration, spinback, bounce)
- Jitter/flutter (triple jitter, high-frequency flutter)
- Complex modulations (amplitude modulation, reverse with multi-gating)
- Swept and ripple patterns (cosine sweep, sawtooth sweep, rapid decay with ripple)

Each gesture is driven by:
- **VelocityCurve**: Position velocity over time (defines playback direction/speed)
- **GatingCurve**: Gain modulation over time (attenuates output)
- **Intensity Parameter**: Scales velocity and modulation depth

### Seeding & Determinism

- Key sample → SHA-256 signature (cryptographic fingerprint)
- Each nibble + key signature → xorshift64* PRNG seed
- PRNG selects gesture type and parameterization
- Same key + nibble always produces identical gesture (reversibility)

## Classification & Matching

### Ensemble Distance Metric

Decoder uses multi-metric classification to robustly match cipher segments:

|             Metric           | Weight |               Purpose                |
|------------------------------|--------|--------------------------------------|
| Normalized Cross-Correlation |  60%   | Robust to amplitude variations       |
| Spectral Band Energy         |  25%   | Captures frequency content (4 bands) |
| Envelope Contour             |  15%   | Captures overall gesture shape       |

Each metric is computed independently, then weighted-averaged for final distance.

### k-NN Voting

When top k nearest neighbors are close in distance (ambiguous match), the algorithm uses robust k-NN voting:
- Finds 3 nearest gesture neighbors
- If top match is significantly better, selects it (margin > 0.1)
- Otherwise, returns median label for robustness

## Limitations

- Mono audio only
- Fixed 44100 Hz sample rate
- Key sample must be at least 1 second long
- Requires exact key for decoding
- Current implementation: 2 nibbles per byte (16 gesture classes active; 32 available for future expansion)
