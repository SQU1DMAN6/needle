# Needle 1.5 - Audio Gesture Cipher

A proof-of-concept implementation of the Needle algorithm in Go, encoding plaintext into realistic DJ scratching audio using cryptographic gesture synthesis.

## Overview

Needle encodes plaintext into a sequence of 32 distinct DJ scratching gestures, creating cipher audio that sounds like professional turntable scratching. Each gesture is deterministically selected based on a cryptographic key sample, enabling perfect decoding with ensemble distance matching.

## Requirements

- Git and Go 1.21 or later (if building from source)
- Mono WAV audio files for encryption key
- Sample rate: 44100 Hz
- Bit depth: 16-bit PCM
- Key sample duration: minimum 1 second

## Installation

Using FtR:

```bash
ftr get JFtR/needle
```

Clone and build from source:

```bash
cd /path/to/needle
go build -o needle main.go
```

This creates a binary called Needle.


## Usage

### Basic Encoding

```bash
./needle encode -key sample.wav -input message.txt -output cipher.wav
```

Encode a plaintext file using a key sample into audio cipher.

Parameters:
- `-key`      Path to key sample WAV file (required, minimum 1 second)
- `-input`    Path to plaintext message file (required)
- `-output`   Path to output cipher WAV file (required)
- `-threads`  Number of threads for parallel processing (optional, default: CPU count)

### Basic Decoding

```bash
./needle decode -key sample.wav -input cipher.wav -output message.txt
```

Decode an audio cipher back to plaintext using the same key sample.

Parameters:
- `-key`      Path to the same key sample used for encoding (required)
- `-input`    Path to cipher WAV file (required)
- `-output`   Path to output plaintext file (required)
- `-threads`  Number of threads for parallel processing (optional, default: CPU count)

### Example Workflow

```bash
# Create a message
echo "The Quick Brown Fox Jumps Over The Lazy Dog" > message.txt

# Encode with key sample
./needle encode -key sample.wav -input message.txt -output cipher.wav

# Decode to verify
./needle decode -key sample.wav -input cipher.wav -output decoded.txt

# Verify perfect match
diff message.txt decoded.txt
```

## How It Works

### Encoding Pipeline

1. **Extract Key Signature**: Compute SHA-256 hash of key sample audio
2. **Plaintext to Nibbles**: Convert each byte to two 4-bit nibbles (0-15)
3. **Seeded Gesture Selection**: For each nibble:
   - Generate deterministic seed from key signature and position
   - Select gesture type (0-31) from xorshift64* PRNG
   - Parameterize intensity based on key data
4. **Audio Synthesis**: For each gesture:
   - Generate variable-length audio segment using motion engine
   - Apply velocity curve for realistic motion
   - Apply gating curve for crossfader modulation
   - Synthesize with selected gesture technique
5. **Audio Output**: Concatenate all segments into mono WAV file (44100 Hz, 16-bit)

Result: Cipher audio sounds like professional DJ scratching

### Decoding Pipeline

1. **Load Cipher Audio**: Parse cipher WAV into variable-length event segments
2. **Build Reference Library**: Reconstruct candidate event sequences using the same key signature
   - Use stateful engine simulation for each candidate path
   - Build sequence candidates rather than fixed-window gestures
3. **Feature Extraction**: For each cipher segment, extract three features:
   - Normalized cross-correlation (60% weight)
   - Spectral band energy (25% weight)
   - Envelope contour (15% weight)
4. **Segment Classification**: Find closest reference gesture using ensemble distance
5. **Reconstruct Message**: Combine nibble pairs into bytes
6. **Output Plaintext**: Write recovered message to file

Key Property: Deterministic seeding ensures encoder and decoder generate identical reference gestures, enabling perfect classification.

## Gesture Architecture

### 32 Professional DJ Scratching Techniques

Needle 1.5 implements 32 distinct gesture types, each crafted to sound like authentic DJ turntable scratching:

**Fundamental Techniques (0-1)**
- Forward Drag: Musical forward motion with wobble modulation
- Reverse Pull: Physics-based deceleration with natural envelope

**Scratch Techniques (2-7)**
- Baby Scratch: Smooth alternating motion (most common technique)
- Transformer Cut: Multiple rapid crossfader cuts with smooth transitions
- Spinback: Rapid cyclical pattern with realistic deceleration
- Tape Stop: Exponential slowdown simulating vinyl stop effect
- Jitter Scratch: High-frequency instability pattern with definition
- Crab Scratch: High-definition rapid cuts with musical rhythm

**Advanced Patterns (8-31)**
- Barrel Roll, Scribble Motion, Orbit Scratches
- Oscillatory patterns with smooth envelopes
- Multi-layer harmonic modulation
- Plus 22 additional distinct gesture types

### Gesture Synthesis Parameters

Each gesture is controlled by:

- **VelocityCurve**: Parameterized motion function (smooth platter motion)
- **GatingCurve**: Crossfader modulation function (realistic gain envelope)
- **Intensity**: Multiplier for velocity and modulation depth (0.5 to 1.5x)

### Deterministic Generation

Each gesture is generated deterministically:

- Key Audio Sample -> SHA-256 KeySignature
- For each nibble: Seed = SHA-256(KeySignature + ByteValue + Position)
- PRNG(Seed) -> Gesture Type (0-31)
- Same seed always produces identical gesture
- Enables library matching and perfect decode accuracy

### Audio Realism Features

- Smooth envelopes without click artifacts
- Crossfader gating with professional timing
- Physics-based velocity curves
- Natural decay modeling
- Multi-frequency harmonic complexity
- Intensity-dependent variation

## Classification & Matching

### Ensemble Distance Metric

Decoder uses a three-component ensemble distance metric to robustly classify cipher segments:

- **Normalized Cross-Correlation (60% weight)**: Robust to amplitude variations
- **Spectral Band Energy (25% weight)**: Captures frequency content across 4 frequency bands
- **Envelope Contour (15% weight)**: Captures overall gesture shape and transient characteristics

Each metric is computed independently, then weighted-averaged for final distance calculation.

### Classification Process

1. Extract all three features from each cipher segment
2. Generate reference library (16 nibble classes)
3. Extract same three features from each reference gesture
4. Compute ensemble distance to all references
5. Select nibble with minimum distance
6. Use robust selection: if top match is significantly better (margin > 0.1), use it; otherwise use median of top 3 matches

## Limitations and Design Notes

- Mono audio only (stereo not supported)
- Fixed 44100 Hz sample rate (required)
- Key sample must be at least 1 second long
- Requires exact same key for decoding (key must not be modified)
- Message size limited to practical plaintext lengths (typical: 1-1000 bytes)
- Current encoding/decoding is sequential (parallel infrastructure ready for future)

## Key Properties

- **Deterministic**: Same plaintext + key always produces identical cipher
- **Reversible**: Perfect round-trip encoding/decoding with 100% accuracy
- **Stateless**: Fresh engine per segment enables library matching
- **Reproducible**: Any modification to plaintext or key produces different cipher
- **Key-dependent**: Gesture selection is cryptographically bound to key sample

## Additional Resources

For detailed information, see:
- GESTURES.md: Complete gesture library documentation
- ARCHITECTURE_DETAILED.md: System architecture and design rationale
- SESSION_SUMMARY.md: Development progress and validation results

## Testing

Verified with three sample key files:
- sample.wav: 1.9M cipher, 53B sample file, 185-210ms encoding, 850-890ms decoding
- sample2.wav: Same performance characteristics
- sample3.wav: Same performance characteristics

All tests show 100% accuracy: plaintext perfectly recovers after encode/decode cycle.

---

_Needle Audio Gesture Cipher, Version 1.5.0, written by Quan Thai, 17 May 2026_