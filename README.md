# Needle 1.6 — Audio Gesture Cipher

> Procedural audio cryptography that sounds like art, not code.
>
> Written by Quan Thai

---

## Philosophy

Needle is not traditional encryption. Traditional encryption destroys structure — it maximises entropy, producing output that is deliberately unrecognisable. Needle does the opposite: it transforms plaintext into procedurally-generated musical performance.

The resulting ciphertext sounds like intentional artistic scratching. It has rhythm, phrasing, energy dynamics, and stylistic vocabulary. It *performs* the plaintext rather than hiding it.

### The Three Priorities

Needle's design is governed by three priorities, in strict order:

#### 1. Artistic Beauty (also first, very necessary)

The ciphertext must sound like a real DJ scratching. Not synthetic noise, not random data — real, musical, expressive turntablism.

This means 32 distinct scratch techniques (baby scratch, transformer cut, crab scratch, scribble, orbit, and 26 more), each with physics-based velocity curves, professional crossfader gating patterns, smooth envelopes with no click artifacts, tempo-aware rhythm (88-119 BPM), and phrase evolution with energy dynamics.

Artistic beauty is a **functional security requirement**. A ciphertext that sounds like noise is suspicious. A ciphertext that sounds like a DJ performance is invisible.

#### 2. Cryptographic Security (first, most necessary)

After beauty, security is the most necessary property. Needle provides:

- **Key conditioning**: SHA-256 binds every gesture to the key sample. Different keys produce completely different output.
- **Deterministic mapping**: Same key + same plaintext = same ciphertext. Always. Guaranteed.
- **Reversibility**: Perfect round-trip encode/decode with 100% accuracy.
- **Interpretation freezing**: Gesture classification happens exactly once at dictionary build time. After that, encode and decode are pure, deterministic operations against a frozen intermediate representation.
- **Zero technique drift**: Decoding a ciphertext and re-encoding the recovered plaintext produces an **identical gesture fingerprint**.

The critical rule: **Stability over expressiveness during encode/decode**. Expressiveness is only allowed during dictionary construction.

#### 3. Speed (last, we'll work on it later)

Speed is deliberately de-prioritised in favour of artistic quality and cryptographic correctness. The dictionary path runs at ~200-250 nibbles/second for encode and ~150-200 nibbles/second for decode. Parallel infrastructure is ready but not yet needed.

---

## Requirements

- Go 1.21 or later
- Mono WAV files at 44100 Hz, 16-bit PCM
- Key sample: minimum 1 second duration
- Linux / macOS / Windows

---

## Installation

```bash
cd src
go build -o needle .
sudo cp needle /usr/local/bin/
```

---

## Quick Start

### Build a dictionary (one-time)

```bash
needle build-dictionary -sample scratch.wav -key sample.wav -output dict.json
```

### Encode a message

```bash
echo "Hello, world" > message.txt
needle encode -key sample.wav -input message.txt -output cipher.wav -dict dict.json
```

### Decode it back

```bash
needle decode -key sample.wav -input cipher.wav -output recovered.txt -dict dict.json
diff message.txt recovered.txt    # identical
```

---

## Practical Use Cases

### Audio Steganography

Hide messages in audio that sounds like intentional musical performance. The ciphertext is indistinguishable from a DJ scratch recording to casual listeners. Without the correct key and dictionary, the plaintext cannot be recovered.

### Procedural Art

Generate expressive cipher audio as an artistic medium. The same plaintext produces different-sounding output with different key samples, creating a unique performance for each key.

### Style Transfer

Extract the technique vocabulary from one recording and use it to encode new messages. The output will use the same scratch techniques in the same proportions as the original.

### Cryptographic Education

Study deterministic synthesis, frozen interpretation, and the relationship between artistic expression and cryptographic security.

---

## Commands

### `build-dictionary`

Build a locked technique dictionary from a reference scratch recording. One-time operation per substrate.

```
needle build-dictionary -sample scratch.wav -key sample.wav -output dict.json [-threshold 0.01]
```

| Flag | Short | Description |
|------|-------|-------------|
| `-sample` | `-S` | Path to reference scratch WAV (required) |
| `-key` | `-K` | Path to key sample WAV (required) |
| `-output` | `-O` | Path for output dictionary JSON (required) |
| `-threshold` | | Frequency threshold λ (default 0.01) |

### `encode`

Encode plaintext into cipher audio. If `-dict`/`-D` is provided, uses the locked dictionary path (deterministic, no technique drift). Without `-dict`, uses the stateful motion engine with PRNG-based gesture selection.

```
needle encode -key sample.wav -input message.txt -output cipher.wav [-dict dict.json]
```

| Flag | Short | Description |
|------|-------|-------------|
| `-key` | `-K` | Path to key sample WAV (required) |
| `-input` | `-I` | Path to plaintext file (required) |
| `-output` | `-O` | Path to output cipher WAV (required) |
| `-dict` | `-D` | Path to locked dictionary JSON (optional) |
| `-q` | | Quiet mode |
| `-qq` | | Verbose mode (only without -dict) |
| `-threads` | | Parallel workers (only without -dict) |

### `decode`

Decode cipher audio to plaintext. If `-dict`/`-D` is provided, uses frame-locked matching (deterministic, no beam search). Without `-dict`, uses beam search over variable-length events.

```
needle decode -key sample.wav -input cipher.wav -output message.txt [-dict dict.json]
```

Same flags as `encode`.

### `inspect`

Analyze a WAV file and output a technique frequency table showing which scratch techniques appear and how often.

```
needle inspect -sample scratch.wav -key sample.wav [-output log.txt]
```

| Flag | Short | Description |
|------|-------|-------------|
| `-sample` | `-S` | Path to WAV to inspect (required) |
| `-key` | `-K` | Path to key sample WAV (required) |
| `-output` | `-O` | Path for raw gesture log output (optional) |

### `validate`

Verify ciphertext consistency by comparing observed gesture log against expected log. Fails hard on any mismatch.

```
needle validate -input cipher.wav -expected-log cipher.wav.gesture_log -key sample.wav -dict dict.json
```

| Flag | Short | Description |
|------|-------|-------------|
| `-input` | `-I` | Cipher WAV to validate (required) |
| `-expected-log` | | Path to expected gesture log (required) |
| `-key` | `-K` | Key sample WAV (required) |
| `-dict` | `-D` | Locked dictionary JSON (required) |

### `version`

```
needle version
```

### `help`

```
needle help
```

---

## Security Considerations

### What Needle Provides

- **Procedural concealment**: Plaintext is embedded in musical structure, not hidden in noise
- **Interpretation ambiguity**: Without the correct dictionary, recovered plaintext appears musically valid but semantically incorrect
- **Key-conditioned synthesis**: All output is deterministically bound to the key sample
- **Zero technique drift**: Gesture fingerprints are preserved across encode/decode/re-encode cycles

### What Needle Does NOT Provide

- Traditional cryptographic security (use AES-256-GCM or ChaCha20-Poly1305 for actual encryption)
- Message authentication
- Plausible deniability
- Quantum resistance

### Intended Use Cases

- **Audio steganography**: Hide small messages in audio that sounds like scratching
- **Procedural art**: Generate expressive cipher audio as an artistic medium
- **Cryptographic education**: Study deterministic synthesis and frozen interpretation
- **Musical watermarking**: Embed gesture vocabulary into recordings

---

## License

Copyright (c) 2026 Quan Thai. See `LICENSE` for details.

---

## Additional Resources

- `ARCHITECTURE.md` — System architecture, data flow diagrams, package map, security model
- `GESTURES.md` — Detailed gesture library documentation (all 32 techniques)

---

*Needle 1.6 — Written by Quan Thai, 2 July 2026*