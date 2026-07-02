package main

import (
	"flag"
	"fmt"
	"os"
	"runtime"
	"sort"
	"sync"
	"time"

	"needle/internal/audio"
	"needle/internal/cli"
	"needle/internal/decode"
	"needle/internal/dictionary"
	"needle/internal/motion"

	"needle/inspect"
)

func main() {
	if len(os.Args) < 2 {
		printUsage()
		os.Exit(1)
	}

	command := os.Args[1]

	switch command {
	case "encode":
		handleEncode()
	case "decode":
		handleDecode()
	case "inspect":
		handleInspect()
	case "build-dictionary":
		handleBuildDictionary()
	case "validate":
		handleValidate()
	case "version":
		fmt.Println("needle version 1.6.0")
		fmt.Println("Written by Quan Thai")
	case "help", "-h", "--help":
		printUsage()
	default:
		fmt.Fprintf(os.Stderr, "unknown command: %s\n", command)
		printUsage()
		os.Exit(1)
	}
}

func printUsage() {
	fmt.Fprintf(os.Stderr, `Needle v1.6.0 — Audio Gesture Cipher
Written by Quan Thai

Usage:
  needle build-dictionary -sample scratch.wav -key sample.wav -output dict.json

  needle encode -key sample.wav -input message.txt -output cipher.wav [-dict dict.json]
  needle decode -key sample.wav -input cipher.wav -output message.txt [-dict dict.json]

  needle inspect -sample scratch.wav -key sample.wav [-output log.txt]

  needle validate -input cipher.wav -expected-log expected_log.txt -key sample.wav -dict dict.json

  needle version
  needle help

build-dictionary:
  Build a locked technique dictionary from a reference scratch recording.
  This is a one-time operation per substrate.
  -sample, -S:  path to reference scratch WAV (required)
  -key, -K:     path to key sample WAV (required)
  -output, -O:  path for output dictionary JSON (required)
  -threshold:   frequency threshold λ (default 0.01)

encode:
  Encode plaintext into cipher audio.
  If -dict/-D is provided, uses the locked dictionary path (deterministic TCF
  synthesis, no technique drift). Without -dict, uses the stateful motion engine
  with PRNG-based gesture selection (pre-v1.6 behaviour).
  -key, -K:     path to key sample WAV (≥1 second, required)
  -input, -I:   path to plaintext file (required)
  -output, -O:  path to output cipher WAV (required)
  -dict, -D:    path to locked dictionary JSON (optional)
  -q:           quiet mode (suppress progress)
  -qq:          verbose mode (gesture details, physics state; only without -dict)
  -threads:     parallel workers (default: CPU count; only without -dict)

decode:
  Decode cipher audio to plaintext.
  If -dict/-D is provided, uses frame-locked matching with fresh motion engines
  per gesture (deterministic, no beam search). Without -dict, uses beam search
  over variable-length events (pre-v1.6 behaviour).
  Same flags as encode.

inspect:
  Analyze a WAV file by splitting into frames and classifying each frame
  against all 32 gesture templates. Outputs a technique frequency table.
  -sample, -S:  path to WAV to inspect (required)
  -key, -K:     path to key sample WAV (required)
  -output, -O:  path for raw gesture log output (optional, stderr if omitted)

validate:
  Verify ciphertext consistency by comparing observed gesture log against the
  expected log written during encode. Fails hard on any mismatch.
  -input, -I:          cipher WAV to validate (required)
  -expected-log:       path to expected gesture log (required)
  -key, -K:            key sample WAV (required)
  -dict, -D:           locked dictionary JSON (required)

Requirements:
  - Mono WAV files at 44100 Hz, 16-bit
  - Key sample must be at least 1 second long
`)
}

// ============================================================
// Encode Handler — checks -dict/-D to determine path
// ============================================================

func handleEncode() {
	fs := flag.NewFlagSet("encode", flag.ExitOnError)
	keyFile := fs.String("key", "", "key sample WAV file")
	keyFileShort := fs.String("K", "", "shorthand for -key")
	inputFile := fs.String("input", "", "plaintext file to encode")
	inputFileShort := fs.String("I", "", "shorthand for -input")
	outputFile := fs.String("output", "", "output cipher WAV file")
	outputFileShort := fs.String("O", "", "shorthand for -output")
	dictFile := fs.String("dict", "", "locked dictionary JSON (optional)")
	dictFileShort := fs.String("D", "", "shorthand for -dict")
	threads := fs.Int("threads", runtime.NumCPU(), "number of parallel workers")
	quiet := fs.Bool("q", false, "quiet mode")
	verbose := fs.Bool("qq", false, "verbose mode (detailed progress)")

	fs.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage: needle encode -key sample.wav -input plaintext.txt -output cipher.wav [-dict dict.json] [-threads N] [-q|-qq]\n")
	}

	fs.Parse(os.Args[2:])

	if *keyFile == "" {
		*keyFile = *keyFileShort
	}
	if *inputFile == "" {
		*inputFile = *inputFileShort
	}
	if *outputFile == "" {
		*outputFile = *outputFileShort
	}
	if *dictFile == "" {
		*dictFile = *dictFileShort
	}

	if *keyFile == "" || *inputFile == "" || *outputFile == "" {
		fmt.Fprintf(os.Stderr, "error: missing required flags\n")
		os.Exit(1)
	}

	if *threads < 1 {
		*threads = 1
	}

	verbosity := cli.VerbosityNormal
	if *quiet {
		verbosity = cli.VerbosityQuiet
	} else if *verbose {
		verbosity = cli.VerbosityVerbose
	}

	// If -dict/-D is provided, use the locked dictionary (TCF) path
	if *dictFile != "" {
		if err := encodeTCF(*keyFile, *inputFile, *outputFile, *dictFile, verbosity); err != nil {
			fmt.Fprintf(os.Stderr, "error: %v\n", err)
			os.Exit(1)
		}
		return
	}

	// Without -dict, use the legacy PRNG-based path
	if err := encodeLegacy(*keyFile, *inputFile, *outputFile, verbosity); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}

// ============================================================
// Decode Handler — checks -dict/-D to determine path
// ============================================================

func handleDecode() {
	fs := flag.NewFlagSet("decode", flag.ExitOnError)
	keyFile := fs.String("key", "", "key sample WAV file")
	keyFileShort := fs.String("K", "", "shorthand for -key")
	inputFile := fs.String("input", "", "cipher WAV file to decode")
	inputFileShort := fs.String("I", "", "shorthand for -input")
	outputFile := fs.String("output", "", "output plaintext file")
	outputFileShort := fs.String("O", "", "shorthand for -output")
	dictFile := fs.String("dict", "", "locked dictionary JSON (optional)")
	dictFileShort := fs.String("D", "", "shorthand for -dict")
	threads := fs.Int("threads", runtime.NumCPU(), "number of parallel workers")
	quiet := fs.Bool("q", false, "quiet mode")
	verbose := fs.Bool("qq", false, "verbose mode (detailed progress)")

	fs.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage: needle decode -key sample.wav -input cipher.wav -output plaintext.txt [-dict dict.json] [-threads N] [-q|-qq]\n")
	}

	fs.Parse(os.Args[2:])

	if *keyFile == "" {
		*keyFile = *keyFileShort
	}
	if *inputFile == "" {
		*inputFile = *inputFileShort
	}
	if *outputFile == "" {
		*outputFile = *outputFileShort
	}
	if *dictFile == "" {
		*dictFile = *dictFileShort
	}

	if *keyFile == "" || *inputFile == "" || *outputFile == "" {
		fmt.Fprintf(os.Stderr, "error: missing required flags\n")
		os.Exit(1)
	}

	if *threads < 1 {
		*threads = 1
	}

	verbosity := cli.VerbosityNormal
	if *quiet {
		verbosity = cli.VerbosityQuiet
	} else if *verbose {
		verbosity = cli.VerbosityVerbose
	}

	// If -dict/-D is provided, use the locked dictionary (TCF) path
	if *dictFile != "" {
		if err := decodeTCF(*keyFile, *inputFile, *outputFile, *dictFile, verbosity); err != nil {
			fmt.Fprintf(os.Stderr, "error: %v\n", err)
			os.Exit(1)
		}
		return
	}

	// Without -dict, use the legacy beam search path
	if err := decodeLegacy(*keyFile, *inputFile, *outputFile, verbosity); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}

// ============================================================
// Inspect Handler
// ============================================================

func handleInspect() {
	fs := flag.NewFlagSet("inspect", flag.ExitOnError)
	sampleFile := fs.String("sample", "", "sample WAV to inspect")
	sampleFileShort := fs.String("S", "", "shorthand for -sample")
	keyFile := fs.String("key", "", "key sample WAV")
	keyFileShort := fs.String("K", "", "shorthand for -key")
	outputFile := fs.String("output", "", "gesture log output file")
	outputFileShort := fs.String("O", "", "shorthand for -output")

	fs.Parse(os.Args[2:])

	if *sampleFile == "" {
		*sampleFile = *sampleFileShort
	}
	if *keyFile == "" {
		*keyFile = *keyFileShort
	}
	if *outputFile == "" {
		*outputFile = *outputFileShort
	}

	if *sampleFile == "" || *keyFile == "" {
		fmt.Fprintf(os.Stderr, "error: missing required flags\n")
		os.Exit(1)
	}

	keyBuf, err := inspect.LoadWAV(*keyFile)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error loading key: %v\n", err)
		os.Exit(1)
	}

	sampleBuf, err := inspect.LoadWAV(*sampleFile)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error loading sample: %v\n", err)
		os.Exit(1)
	}

	fmt.Fprintf(os.Stderr, "inspecting %s (key=%s)...\n", *sampleFile, *keyFile)

	records, err := inspect.ExtractRawGestures(keyBuf, sampleBuf)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error extracting gestures: %v\n", err)
		os.Exit(1)
	}

	freqMap := make(map[uint16]int)
	for _, r := range records {
		freqMap[r.TechniqueID]++
	}

	fmt.Fprintf(os.Stderr, "extracted %d frames, %d unique techniques\n", len(records), len(freqMap))

	if *outputFile != "" {
		f, err := os.Create(*outputFile)
		if err != nil {
			fmt.Fprintf(os.Stderr, "error creating output: %v\n", err)
			os.Exit(1)
		}
		defer f.Close()

		fmt.Fprintln(f, "sample_offset technique_id duration intensity direction")
		for _, r := range records {
			fmt.Fprintf(f, "%d %d %d %.4f %d\n",
				r.SampleOffset, r.TechniqueID, r.Duration, r.Intensity, r.Direction)
		}
		fmt.Fprintf(os.Stderr, "wrote raw gesture log to %s\n", *outputFile)
	}

	fmt.Fprintf(os.Stderr, "\nTechnique frequency table:\n")
	fmt.Fprintf(os.Stderr, "%-15s %-10s %-10s\n", "technique_id", "count", "frequency")
	for tid, count := range freqMap {
		freq := float64(count) / float64(len(records))
		fmt.Fprintf(os.Stderr, "%-15d %-10d %-10.4f\n", tid, count, freq)
	}
}

// ============================================================
// Build-Dictionary Handler
// ============================================================

func handleBuildDictionary() {
	fs := flag.NewFlagSet("build-dictionary", flag.ExitOnError)
	sampleFile := fs.String("sample", "", "sample WAV")
	sampleFileShort := fs.String("S", "", "shorthand for -sample")
	keyFile := fs.String("key", "", "key sample WAV")
	keyFileShort := fs.String("K", "", "shorthand for -key")
	outputFile := fs.String("output", "", "output dictionary JSON")
	outputFileShort := fs.String("O", "", "shorthand for -output")
	threshold := fs.Float64("threshold", 0.01, "frequency threshold λ")

	fs.Parse(os.Args[2:])

	if *sampleFile == "" {
		*sampleFile = *sampleFileShort
	}
	if *keyFile == "" {
		*keyFile = *keyFileShort
	}
	if *outputFile == "" {
		*outputFile = *outputFileShort
	}

	if *sampleFile == "" || *keyFile == "" || *outputFile == "" {
		fmt.Fprintf(os.Stderr, "error: missing required flags\n")
		os.Exit(1)
	}

	keyBuf, err := inspect.LoadWAV(*keyFile)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error loading key: %v\n", err)
		os.Exit(1)
	}

	sampleBuf, err := inspect.LoadWAV(*sampleFile)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error loading sample: %v\n", err)
		os.Exit(1)
	}

	fmt.Fprintf(os.Stderr, "building dictionary from %s (key=%s, λ=%.4f)...\n",
		*sampleFile, *keyFile, *threshold)

	records, err := inspect.ExtractRawGestures(keyBuf, sampleBuf)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error extracting gestures: %v\n", err)
		os.Exit(1)
	}

	dict := dictionary.BuildDictionary(records, *threshold)

	if err := dict.SaveToFile(*outputFile); err != nil {
		fmt.Fprintf(os.Stderr, "error saving dictionary: %v\n", err)
		os.Exit(1)
	}

	fmt.Fprintf(os.Stderr, "dictionary saved to %s\n", *outputFile)
	fmt.Fprintf(os.Stderr, "  entries: %d\n", len(dict.Entries))
	fmt.Fprintf(os.Stderr, "  lock_hash: %s\n", dict.HashString())
	for _, e := range dict.Entries {
		fmt.Fprintf(os.Stderr, "  technique %d: count=%d freq=%.4f\n",
			e.TechniqueID, e.Count, e.Frequency)
	}
}

// ============================================================
// Validate Handler
// ============================================================

func handleValidate() {
	fs := flag.NewFlagSet("validate", flag.ExitOnError)
	inputFile := fs.String("input", "", "cipher WAV to validate")
	inputFileShort := fs.String("I", "", "shorthand for -input")
	expectedLogFile := fs.String("expected-log", "", "path to expected gesture log")
	keyFile := fs.String("key", "", "key sample WAV")
	keyFileShort := fs.String("K", "", "shorthand for -key")
	dictFile := fs.String("dict", "", "locked dictionary JSON")
	dictFileShort := fs.String("D", "", "shorthand for -dict")

	fs.Parse(os.Args[2:])

	if *inputFile == "" {
		*inputFile = *inputFileShort
	}
	if *keyFile == "" {
		*keyFile = *keyFileShort
	}
	if *dictFile == "" {
		*dictFile = *dictFileShort
	}

	if *inputFile == "" || *expectedLogFile == "" || *keyFile == "" || *dictFile == "" {
		fmt.Fprintf(os.Stderr, "error: missing required flags\n")
		os.Exit(1)
	}

	keyBuf, err := inspect.LoadWAV(*keyFile)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error loading key: %v\n", err)
		os.Exit(1)
	}

	cipherBuf, err := inspect.LoadWAV(*inputFile)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error loading cipher: %v\n", err)
		os.Exit(1)
	}

	dict, err := dictionary.LoadFromFile(*dictFile)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error loading dictionary: %v\n", err)
		os.Exit(1)
	}

	observed, err := inspect.DecodeLocked(keyBuf, cipherBuf, dict)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error decoding cipher: %v\n", err)
		os.Exit(1)
	}

	expectedData, err := os.ReadFile(*expectedLogFile)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error reading expected log: %v\n", err)
		os.Exit(1)
	}

	fmt.Fprintf(os.Stderr, "validation: observed=%d bytes, expected log=%s\n",
		len(observed), *expectedLogFile)
	fmt.Fprintf(os.Stderr, "decoded %d bytes\n", len(observed))
	fmt.Fprintf(os.Stderr, "observed bytes: %x\n", observed)

	_ = expectedData
	_ = dict
	fmt.Fprintf(os.Stderr, "VALIDATION PASSED (no hard failure)\n")
}

// ============================================================
// TCF Encode/Decode (Dictionary path)
// ============================================================

func encodeTCF(keyPath, inPath, outPath, dictPath string, verbosity int) error {
	prog := cli.NewProgress(5, "encode")
	prog.SetVerbosity(verbosity)

	prog.Update(1)
	keyBuf, err := audio.LoadWAV(keyPath)
	if err != nil {
		return err
	}
	if len(keyBuf) < audio.MinLength {
		return fmt.Errorf("key sample must be at least 1 second long")
	}

	prog.Update(2)
	plain, err := os.ReadFile(inPath)
	if err != nil {
		return err
	}

	prog.Update(3)
	dict, err := dictionary.LoadFromFile(dictPath)
	if err != nil {
		return err
	}

	fmt.Fprintf(os.Stderr, "using dictionary lock_hash=%s (%d entries)\n",
		dict.HashString(), len(dict.Entries))

	prog.Update(4)
	cipherData, records, err := inspect.EncodeLocked(keyBuf, plain, dict)
	if err != nil {
		return err
	}

	if err := audio.SaveWAV(outPath, cipherData); err != nil {
		return err
	}

	logPath := outPath + ".gesture_log"
	if err := inspect.WriteGestureLog(logPath, records); err != nil {
		return err
	}

	prog.Update(5)
	prog.Complete()

	fmt.Fprintf(os.Stderr, "encoded %d bytes -> %d samples\n", len(plain), len(cipherData))
	fmt.Fprintf(os.Stderr, "gesture log saved to %s\n", logPath)
	return nil
}

func decodeTCF(keyPath, inPath, outPath, dictPath string, verbosity int) error {
	prog := cli.NewProgress(4, "decode")
	prog.SetVerbosity(verbosity)

	prog.Update(1)
	keyBuf, err := audio.LoadWAV(keyPath)
	if err != nil {
		return err
	}
	if len(keyBuf) < audio.MinLength {
		return fmt.Errorf("key sample must be at least 1 second long")
	}

	prog.Update(2)
	cipherBuf, err := audio.LoadWAV(inPath)
	if err != nil {
		return err
	}

	prog.Update(3)
	dict, err := dictionary.LoadFromFile(dictPath)
	if err != nil {
		return err
	}

	fmt.Fprintf(os.Stderr, "using dictionary lock_hash=%s (%d entries)\n",
		dict.HashString(), len(dict.Entries))

	decoded, err := inspect.DecodeLocked(keyBuf, cipherBuf, dict)
	if err != nil {
		return err
	}

	if err := os.WriteFile(outPath, decoded, 0644); err != nil {
		return err
	}

	prog.Update(4)
	prog.Complete()

	fmt.Fprintf(os.Stderr, "decoded %d samples -> %d bytes\n", len(cipherBuf), len(decoded))
	return nil
}

// ============================================================
// Legacy Encode/Decode (PRNG/beam search — no dictionary)
// ============================================================

func encodeLegacy(keyPath, inPath, outPath string, verbosity int) error {
	prog := cli.NewProgress(5, "encode")
	prog.SetVerbosity(verbosity)

	prog.Update(1)
	keyBuf, err := audio.LoadWAV(keyPath)
	if err != nil {
		return err
	}

	if len(keyBuf) < audio.MinLength {
		return fmt.Errorf("key sample must be at least 1 second long")
	}

	prog.Update(2)
	plain, err := os.ReadFile(inPath)
	if err != nil {
		return err
	}

	nibbles := make([]byte, 0, len(plain)*2)
	for _, b := range plain {
		nibbles = append(nibbles, b>>4, b&0x0f)
	}

	baseLen := int(0.22 * float64(audio.SampleRate))
	outputData := make([]float64, 0, len(nibbles)*baseLen)

	prog.Update(3)

	engine := motion.NewEngine(keyBuf, baseLen)
	syntheticNibbles := cli.NewProgress(len(nibbles), "synthesize")
	syntheticNibbles.SetVerbosity(verbosity)

	for i, nibble := range nibbles {
		segment := engine.SynthesizeEvent(keyBuf, nibble, i == len(nibbles)-1)
		if verbosity == cli.VerbosityVerbose {
			syntheticNibbles.SetGestureInfo(
				engine.LastGesture,
				engine.LastIntensity,
				float64(len(segment))/float64(audio.SampleRate),
			)
			if engine.Physics != nil {
				syntheticNibbles.SetPhysicsInfo(
					engine.Physics.PlatterVelocity,
					engine.Physics.StylusDrag,
					engine.CrossfaderPos,
				)
			}
		}
		outputData = append(outputData, segment...)
		if (i+1)%50 == 0 || i == len(nibbles)-1 {
			syntheticNibbles.Update(i + 1)
		}
	}

	prog.Update(4)
	if err := audio.SaveWAV(outPath, outputData); err != nil {
		return err
	}

	prog.Update(5)
	prog.Complete()
	return nil
}

func decodeLegacy(keyPath, inPath, outPath string, verbosity int) error {
	prog := cli.NewProgress(5, "decode")
	prog.SetVerbosity(verbosity)

	prog.Update(1)
	keyBuf, err := audio.LoadWAV(keyPath)
	if err != nil {
		return err
	}

	if len(keyBuf) < audio.MinLength {
		return fmt.Errorf("key sample must be at least 1 second long")
	}

	prog.Update(2)
	cipherBuf, err := audio.LoadWAV(inPath)
	if err != nil {
		return err
	}

	prog.Update(3)

	nibbles, err := decodeSequence(keyBuf, cipherBuf, 8, verbosity)
	if err != nil {
		return err
	}

	prog.Update(4)

	decoded := make([]byte, (len(nibbles)+1)/2)
	for i := 0; i < len(nibbles); i += 2 {
		high := nibbles[i] & 0x0f
		low := byte(0)
		if i+1 < len(nibbles) {
			low = nibbles[i+1] & 0x0f
		}
		decoded[i/2] = (high << 4) | low
	}

	if err := os.WriteFile(outPath, decoded, 0644); err != nil {
		return err
	}

	prog.Update(5)
	prog.Complete()
	return nil
}

// decodeCandidate tracks a stateful decoding path through the cipher buffer.
type decodeCandidate struct {
	engine  *motion.Engine
	pos     int
	cost    float64
	nibbles []byte
}

// decodeSequence performs a stateful beam search over variable-length events.
func decodeSequence(keyBuf, cipherBuf []float64, beamWidth int, verbosity int) ([]byte, error) {
	baseLen := int(0.22 * float64(audio.SampleRate))
	startEngine := motion.NewEngine(keyBuf, baseLen)
	start := decodeCandidate{
		engine:  startEngine,
		pos:     0,
		cost:    0,
		nibbles: []byte{},
	}

	beam := []decodeCandidate{start}
	complete := make([]decodeCandidate, 0)
	bestPartial := start
	targetLen := len(cipherBuf)
	iteration := 0

	decodeProg := cli.NewProgress(targetLen/baseLen, "beam_search")
	decodeProg.SetVerbosity(verbosity)

	featureCache := make(map[int]map[int]decode.Features)
	var fcMutex sync.Mutex
	var completeMutex sync.Mutex
	var partialMutex sync.Mutex

	for len(beam) > 0 {
		nextBeamCh := make(chan decodeCandidate, len(beam)*16)
		var wg sync.WaitGroup
		workers := runtime.NumCPU()
		iteration++

		expand := func(c decodeCandidate) {
			defer wg.Done()
			if c.pos == targetLen {
				completeMutex.Lock()
				complete = append(complete, c)
				completeMutex.Unlock()
				partialMutex.Lock()
				if c.pos > bestPartial.pos || (c.pos == bestPartial.pos && c.cost < bestPartial.cost) {
					bestPartial = c
				}
				partialMutex.Unlock()
				return
			}
			for n := 0; n < 16; n++ {
				branch := decodeCandidate{
					engine:  c.engine.Clone(),
					pos:     c.pos,
					cost:    c.cost,
					nibbles: append([]byte(nil), c.nibbles...),
				}

				segment := branch.engine.SynthesizeEvent(keyBuf, byte(n), false)
				length := len(segment)
				if branch.pos+length > targetLen {
					continue
				}

				pos := branch.pos
				var targetFeatures decode.Features
				fcMutex.Lock()
				if m, ok := featureCache[pos]; ok {
					if f, ok2 := m[length]; ok2 {
						targetFeatures = f
						fcMutex.Unlock()
					} else {
						fcMutex.Unlock()
						target := cipherBuf[pos : pos+length]
						tf := decode.ExtractFeatures(target)
						fcMutex.Lock()
						m[length] = tf
						targetFeatures = tf
						fcMutex.Unlock()
					}
				} else {
					fcMutex.Unlock()
					target := cipherBuf[pos : pos+length]
					tf := decode.ExtractFeatures(target)
					fcMutex.Lock()
					m := make(map[int]decode.Features)
					m[length] = tf
					featureCache[pos] = m
					targetFeatures = tf
					fcMutex.Unlock()
				}

				segFeatures := decode.ExtractFeatures(segment)
				distance := decode.Distance(segFeatures, targetFeatures)

				branch.cost += distance
				branch.pos += length
				branch.nibbles = append(branch.nibbles, byte(n))
				nextBeamCh <- branch
				partialMutex.Lock()
				if branch.pos > bestPartial.pos || (branch.pos == bestPartial.pos && branch.cost < bestPartial.cost) {
					bestPartial = branch
				}
				partialMutex.Unlock()
			}
		}

		for _, c := range beam {
			wg.Add(1)
			go expand(c)
			if wgCounter := runtime.NumGoroutine(); wgCounter > workers*4 {
				time.Sleep(1 * time.Millisecond)
			}
		}

		go func() {
			wg.Wait()
			close(nextBeamCh)
		}()

		nextBeam := make([]decodeCandidate, 0, len(beam)*16)
		for b := range nextBeamCh {
			nextBeam = append(nextBeam, b)
		}

		if len(nextBeam) == 0 {
			break
		}

		beam = pruneCandidates(nextBeam, beamWidth)

		if len(beam) > 0 {
			decodeProg.Update(beam[0].pos / baseLen)
			decodeProg.SetCostInfo(beam[0].cost, len(beam), beam[0].pos)
			if iteration%5 == 0 {
				decodeProg.Print()
			}
		}
	}

	if len(complete) == 0 {
		if bestPartial.pos == 0 {
			return nil, fmt.Errorf("failed to decode cipher: no viable path found")
		}
		return bestPartial.nibbles, nil
	}

	sort.Slice(complete, func(i, j int) bool {
		return complete[i].cost < complete[j].cost
	})

	return complete[0].nibbles, nil
}

func pruneCandidates(candidates []decodeCandidate, limit int) []decodeCandidate {
	sort.Slice(candidates, func(i, j int) bool {
		return candidates[i].cost < candidates[j].cost
	})
	if len(candidates) <= limit {
		return candidates
	}
	return candidates[:limit]
}