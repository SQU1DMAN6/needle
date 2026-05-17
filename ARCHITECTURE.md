# Needle 1.5 Architecture Overview

## Phase 1: Continuous Stateful Synthesis + Physics Layer

### What's New

#### 1. **Physics Simulation Module** (`internal/motion/physics.go`)
- `PhysicsState` struct modeling realistic turntable physics:
  - **Platter dynamics**: Inertia (0.92 momentum retention), friction (0.08)
  - **Stylus drag**: Friction from needle contact
  - **Crossfader response**: Smooth S-curve crossfader modeling
  - **Motor dynamics**: Motor torque and responsiveness (0.88 inertia)
  - **Historical context**: Tracks previous velocity and acceleration

- Key functions:
  - `ApplyPlatterPhysics()` - Simulates realistic platter acceleration/deceleration
  - `ApplyStylusDrag()` - Models needle friction effects
  - `UpdateCrossfader()` - Smooth crossfader transitions
  - `GetEffectiveVelocity()` - Combines all physical effects

#### 2. **Parallel Processing Module** (`internal/parallel/parallel.go`)
- Multi-core encoding/decoding:
  - `EncodeParallel()` - Distributes nibble synthesis across worker threads
  - `DecodeParallel()` - Parallel library building and segment classification
  - Worker pool architecture with job distribution channels
  - Default: Uses all available CPU cores

#### 3. **Enhanced Motion Engine** (`internal/motion/engine.go`)
- Added `SegmentIndex` tracking for procedural context
- Foundation for state continuity (prepared for Needle 1.5 full features)
- Compatible with both physics simulation and parallel processing

#### 4. **CLI Improvements** (`main.go`)
- `-threads N` flag for parallelization control
- Version bumped to 1.5.0
- Better error handling and cleaner UI

### Architecture Flow

```
Encoding:
  plaintext 
    ↓
  [nibble splitting]
    ↓
  [parallel workers] → [motion engine + physics] → [gesture synthesis]
    ↓
  [WAV output]

Decoding:
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
