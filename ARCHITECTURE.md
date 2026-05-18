# Needle 1.5 Stable - Architecture Document

## System Overview

Needle 1.5 Stable is a stateful procedural audio synthesis system that encodes plaintext into realistic DJ scratching audio. The core innovation is **performance-context-aware gesture synthesis**, where each gesture depends not only on the plaintext byte but also on accumulated performance state (tempo, phrase position, energy, momentum, gesture history).

### Design Principles

1. **Statefulness**: Every gesture influences future synthesis through persistent physics state
2. **Musical Coherence**: Gestures are rhythmically aware and contextually sensitive
3. **Procedural Ambiguity**: Without the correct key, plausible alternative interpretations exist
4. **Security Through Structure**: Gesture dependencies prevent isolated waveform analysis

## Major Components

### 1. Motion Engine (`internal/motion/engine.go`)

The core synthesis engine that generates audio events from plaintext bytes.

#### State Management

```go
type Engine struct {
    KeySignature   uint64        // Deterministic key fingerprint
    State          *State        // Continuous position/velocity
    Tempo          float64       // 88-119 BPM from key
    BeatPosition   float64       // 0-4 (phrase tracking)
    PhraseEnergy   float64       // 0-1 (expression level)
    CrossfaderPos  float64       // 0-1 (mixer state)
    LastGesture    int           // Previous gesture type (for continuity)
    LastIntensity  float64       // Previous gesture intensity
    Physics        *PhysicsState // Platter/stylus dynamics
}
```

**KeySignature**: Computed from key sample via SHA256. Used in seed generation to ensure key-dependent behavior.

**Tempo**: Extracted from key signature (88-119 BPM range). Affects event duration and groove sensitivity.

**BeatPosition**: Tracks position within 4-beat phrase. Influences:
- Event duration (longer at phrase boundaries for musical closure)
- Gesture intensity (builds and releases with phrase structure)
- Phrase energy evolution

**PhraseEnergy**: Smoothly tracks plaintext intensity patterns. Range [0.1, 1.0]:
- Low energy: softer, shorter gestures
- High energy: intense, longer scratching
- Creates natural "musical pacing" without explicit segmentation

**Physics State**: Carries over platter momentum, stylus drag, crossfader inertia across events. Prevents "staccato" artifact where each gesture sounds disconnected.

#### Event Duration (Variable Timing)

Traditional fixed-length encoding (160-250ms per nibble) leaks segmentation. Needle 1.5 uses **rhythm-aware variable timing**:

```go
func (e *Engine) EventLen(byteVal byte) int {
    beatSamples := float64(44100) * 60.0 / e.Tempo
    subdivisions := []float64{0.5, 0.75, 1.0, 1.25, 1.5}  // Beat fractions
    
    // Deterministic selection from byte + phrase position
    seed := crypto.SeedGen(...)
    sub := subdivisions[seed.NextInt(5)]
    
    // Phrase-boundary extension (musical closure)
    phraseFactor := 1.0
    if e.BeatPosition > 3.0 {
        phraseFactor = 1.15  // ~15% longer at phrase end
    }
    
    length := int(sub * beatSamples * phraseFactor)
    // Clamp to 150-400ms for realistic, faster events
    return clamp(length, 150*44.1, 400*44.1)
}
```

**Advantages**:
- Segmentation not visible in audio length patterns
- Musical phrasing hides timing structure
- Prevents deterministic offset-based attacks
- Sounds like intentional scratching, not segmented encoding

#### Gesture Synthesis (`SynthesizeEvent`)

For each plaintext byte (or nibble):

1. **Determine event length** based on byte and phrase position
2. **Generate seed** from key + byte + position + performance context
3. **Select gesture type** (0-31) from seed
4. **Apply physics**: platter inertia, stylus drag, crossfader smoothing
5. **Synthesize segment**: loop through position/velocity, apply gating
6. **Update performance state**: beat position, phrase energy, gesture history

#### Security Through State Mixing

Each gesture synthesis depends on:
- **Plaintext byte**: Core data
- **Position**: Prevents reordering
- **Phrase beat**: Prevents timing attacks
- **Phrase energy**: Prevents energy-pattern analysis
- **Last gesture**: Enforces sequence continuity
- **Physics state**: Ensures smooth transitions

Result: No two identical bytes produce identical audio (context-dependent).

### 2. Physics State (`internal/motion/physics.go`)

Realistic turntable dynamics prevent synthetic staccato artifacts.

#### Platter Inertia

Smooth motion requires momentum carryover:

```go
PlatterVelocity *= PlatterInertia  // 92% momentum retention per sample
Acceleration = clamp(acceleration, -0.15, 0.15)  // Realistic limits
```

When gesture synthesis commands velocity changes, the platter doesn't snap instantly. Instead, it smoothly accelerates based on motor torque and inertia, creating realistic DJ scratching motion.

#### Stylus Drag

Friction from stylus contact opposes platter motion:

```go
StylusDrag = sign(PlatterVelocity) * dragFactor
```

Creates resistance proportional to stylus pressure, making scratches feel "weighty" instead of synthetic.

#### Crossfader Dynamics

Real crossfaders have mechanical inertia:

```go
CrossfaderPos += posError * 0.15  // Smooth response, not instant
```

Prevents instantaneous fading, creating smooth transitions instead of harsh cuts.

#### Momentum Influence

Gesture selection is influenced by platter momentum:

```go
MomentumInfluence = math.Abs(PreviousVelocity) * 0.4
```

High momentum from previous scratch influences next gesture selection, creating performance continuity.

### 3. Gesture Policy (`internal/gesture/policy.go`)

32 expressive scratching techniques with context-aware selection.

#### Gesture Types (0-31)

Each gesture defines:
- **VelocityCurve**: Platter motion over gesture duration (0-1 normalized time)
- **GatingCurve**: Crossfader behavior (cuts, fades, sustain)
- **Intensity**: How aggressive/expressive the gesture is

**Realistic Frequencies**:
- Baby Scratch: 3-5 Hz (was 18-26 Hz in 1.5 Dirty)
- Crab Scratch: 5-8 Hz (was 28-53 Hz)
- Transformer: 2-4 cuts (was 4-7)
- Jitter: 6-10 Hz (was 30-50 Hz)

These match real DJ scratching, eliminating the "beep/distortion" artifacts.

#### Context-Aware Selection

Gesture selection depends on:
- Plaintext byte
- Phrase position (beat phase)
- Phrase energy (accumulated motion)
- Previous gesture type (prevents repetition)

**Security Effect**: Sequence-level dependencies prevent isolated waveform equivalence attacks.

### 4. Cryptographic Foundation (`internal/crypto/seed.go`)

#### Enhanced Seed Generation

```go
func SeedGenWithContext(keySignature, phraseBeat, energy, lastGesture) uint64 {
    // Mix key + position + byte + context into SHA256
    // 40 bytes total input for strong mixing
}
```

Gesture selection now depends on:
- Key signature
- Plaintext byte
- Position (prevents reordering)
- Phrase beat (prevents timing attacks)
- Phrase energy (prevents energy analysis)
- Last gesture (prevents sequence guessing)

Each context change means platter velocity changes, affecting phase alignment. Without correct key + context, seed guessing fails.

### 5. Decoder (`main.go` - `decodeSequence`)

Beam search interprets gesture sequences as performance continuity:

```go
for len(beam) > 0 {
    for candidate := range beam {
        for nibble := 0; nibble < 16; nibble++ {
            segment := engine.SynthesizeEvent(keyBuf, byte(nibble))
            distance := decode.Distance(segment, cipherBuf[...])
            candidate.cost += distance
        }
    }
    beam = topN(nextBeam, beamWidth)
}
```

**Stateful Search**: Each beam candidate maintains complete engine state. Incorrect paths accumulate higher cost because engine state desynchronizes.

## Security Analysis

### What Needle Protects Against

1. **Waveform Classification**: Gestures sound musical; their purpose is ambiguous
2. **Segmentation Analysis**: Variable timing (300-700ms) hides encoding boundaries
3. **Gesture Equivalence**: State-dependent synthesis prevents "byte X → gesture Y" patterns
4. **Timing Attacks**: Phrase structure masks plaintext patterns
5. **Isolated Decoding**: Gestures depend on context; can't decode single events

### What Needle Does NOT Protect Against

1. **Key Recovery**: With sufficient ciphertext, optimization may recover key
2. **Statistical Analysis**: Plaintext patterns may emerge in gesture distributions
3. **Metadata Leakage**: Ciphertext length directly relates to plaintext length
4. **Authentication**: No message authentication
5. **Quantum Attacks**: SHA256 is not quantum-resistant

### Intended Threat Model

Needle assumes:
- Key sample is kept secret
- Ciphertext may be public
- Attacker cannot intercept key exchange
- Attacker has no plaintext/ciphertext pairs (or very few)

**Goal**: Make recovered plaintext indistinguishable from plausible alternative interpretations.

## Performance Characteristics

### Encoding

- Gesture synthesis: O(n_nibbles * event_length)
- Event length: 150-400ms (6,615-17,640 samples at 44.1 kHz)
- Rate: ~50-100 nibbles/second
- Memory: Minimal (streaming synthesis)

### Decoding

- Beam search: O(n_candidates * 16 * feature_extraction)
- Beam width: 8 (tunable)
- Improvements: parallel beam expansion and a target feature cache reduce redundant work; tune `-threads` for best throughput
- Rate: variable depending on CPU and beam width (small tests: ~5-30 nibbles/second)
- Memory: O(beamWidth * state_size)

### Audio Quality

- Sample rate: 44100 Hz
- Bit depth: 16-bit PCM
- Channels: Mono
- File size: ~88 KB per second

## Recent Improvements (Needle 1.5 Stable)

### Realism (Issue #1)
- ✅ Gesture duration: 150-400ms (was 300-700ms in earlier drafts)
- ✅ Increased base intensity and velocity response for punchier, more aggressive scratching (inspired by Sid Wilson / Craig Jones)
- ✅ Oscillation frequencies reduced 2-4x (baby scratch: 3-5 Hz not 18 Hz)
- ✅ Fewer crossfader cuts (3-5 not 7-12)
- ✅ Eliminated high-frequency "beep" artifacts

### Progress Reporting (Issue #2)
- ✅ Added `-q` (quiet) and `-qq` (verbose) flags
- ✅ Shows gesture type, intensity, duration
- ✅ Shows physics state (platter velocity, stylus drag, crossfader)
- ✅ Shows beam search cost and position during decoding

### Security (Issue #3)
- ✅ Context-aware seed generation (includes phrase beat, energy, last gesture)
- ✅ Gesture selection influenced by performance state
- ✅ State mixing prevents isolated gesture equivalence
- ✅ Sequence dependencies enforce continuity

## Future Extensions

### Short-Term (Needle 1.6)
- Stereo scratching (dual-channel, higher bitrate)
- Neural feature extraction (improved decoder accuracy)
- Real-time processing

### Medium-Term (Needle 2.0)
- Genre-aware synthesis (house, drum & bass, turntablism)
- Phrase generation (multi-nibble motifs with melody)
- Adaptive beam search

### Long-Term (Needle 3.0)
- Full neural audio synthesis (learned from DJ recordings)
- Multi-layer encoding (main + steganographic watermarks)
- Adversarial robustness (transformation-resistant)

---

**Needle 1.5 Stable**: Procedural audio synthesis meets cryptographic ambiguity.

  cipher.wav
    ↓
  [parallel workers] → [build library with physics]
    ↓
  [parallel classification] → [feature matching]
    ↓
  [nibble reconstruction] → plaintext
```

### Security & Performance

- **Speed**: Multi-core processing (typically 2-8x faster depending on CPU)
- **Realism**: Physics-based synthesis makes audio sound like real DJ scratching
- **Determinism**: Maintained perfect round-trip encode/decode accuracy
- **Security**: Foundation set for Needle 1.5 ambiguity layer (optional enhancement)

### Current Mode

- **Needle 1.0 Compatibility Mode**: Working perfectly with all sample keys
- **100% round-trip accuracy** maintained
- Physics layer ready for optional state-continuity features (Phase 2)

### Future Enhancements

1. **Dynamic Variable-Length Segments** - Remove fixed 250ms duration
2. **Continuous State Carryover** - Enable smooth gesture transitions
3. **Ambiguity Injection** - Add controlled feature-space overlap
4. **Advanced Gesture Morphing** - Seed-dependent gesture evolution
