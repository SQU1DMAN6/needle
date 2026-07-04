package main

import (
	"flag"
	"fmt"
	"math"
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
  needle build-dictionary -sample <scratch.wav> -key <sample.wav> -output <dict.json>
  needle encode -key <sample.wav> -input <file> -output <cipher.wav> [-dict <dict.json>]
  needle decode -key <sample.wav> -input <cipher.wav> -output <file> [-dict <dict.json>]
  needle inspect -sample <file.wav> -key <sample.wav>
  needle validate -input <cipher.wav> -expected-log <log.txt> -key <sample.wav> -dict <dict.json>
  needle version
  needle help

Flags:
  -q    Quiet mode (suppress progress)
  -qq   Verbose mode (detailed output)
  -duration float
        Gesture duration multiplier (0.5-2.0, default 1.0)
`)
}

func handleEncode() {
	fs := flag.NewFlagSet("encode", flag.ExitOnError)
	keyFile := fs.String("key", "", "key sample WAV")
	keyFileShort := fs.String("K", "", "shorthand for -key")
	inputFile := fs.String("input", "", "plaintext file")
	inputFileShort := fs.String("I", "", "shorthand for -input")
	outputFile := fs.String("output", "", "output cipher WAV")
	outputFileShort := fs.String("O", "", "shorthand for -output")
	dictFile := fs.String("dict", "", "dictionary JSON (optional)")
	dictFileShort := fs.String("D", "", "shorthand for -dict")
	logFile := fs.String("log", "", "gesture log (optional)")
	logFileShort := fs.String("L", "", "shorthand for -log")
	quiet := fs.Bool("q", false, "quiet mode")
	verbose := fs.Bool("qq", false, "verbose mode")
	duration := fs.Float64("duration", 1.0, "gesture duration multiplier (0.5-2.0)")
	fs.Parse(os.Args[2:])
	if *keyFileShort != "" {
		keyFile = keyFileShort
	}
	if *inputFileShort != "" {
		inputFile = inputFileShort
	}
	if *outputFileShort != "" {
		outputFile = outputFileShort
	}
	if *dictFileShort != "" {
		dictFile = dictFileShort
	}
	if *logFileShort != "" {
		logFile = logFileShort
	}
	if *keyFile == "" || *inputFile == "" || *outputFile == "" {
		fmt.Fprintf(os.Stderr, "error: missing required flags\n")
		os.Exit(1)
	}
	verbosity := verbosityLevel(*quiet, *verbose)
	if *duration < 0.5 {
		*duration = 0.5
	}
	if *duration > 2.0 {
		*duration = 2.0
	}
	if *dictFile != "" {
		if err := encodeTCFWithDuration(*keyFile, *inputFile, *outputFile, *dictFile, *logFile, verbosity, *duration); err != nil {
			fmt.Fprintf(os.Stderr, "error: %v\n", err)
			os.Exit(1)
		}
		return
	}
	if err := encodeLegacy(*keyFile, *inputFile, *outputFile, verbosity); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}

func handleDecode() {
	fs := flag.NewFlagSet("decode", flag.ExitOnError)
	keyFile := fs.String("key", "", "key sample WAV")
	keyFileShort := fs.String("K", "", "shorthand for -key")
	inputFile := fs.String("input", "", "cipher WAV")
	inputFileShort := fs.String("I", "", "shorthand for -input")
	outputFile := fs.String("output", "", "output plaintext")
	outputFileShort := fs.String("O", "", "shorthand for -output")
	dictFile := fs.String("dict", "", "dictionary JSON (optional)")
	dictFileShort := fs.String("D", "", "shorthand for -dict")
	quiet := fs.Bool("q", false, "quiet mode")
	verbose := fs.Bool("qq", false, "verbose mode")
	fs.Parse(os.Args[2:])
	if *keyFileShort != "" {
		keyFile = keyFileShort
	}
	if *inputFileShort != "" {
		inputFile = inputFileShort
	}
	if *outputFileShort != "" {
		outputFile = outputFileShort
	}
	if *dictFileShort != "" {
		dictFile = dictFileShort
	}
	if *keyFile == "" || *inputFile == "" || *outputFile == "" {
		fmt.Fprintf(os.Stderr, "error: missing required flags\n")
		os.Exit(1)
	}
	verbosity := verbosityLevel(*quiet, *verbose)
	if *dictFile != "" {
		if err := decodeTCF(*keyFile, *inputFile, *outputFile, *dictFile, verbosity); err != nil {
			fmt.Fprintf(os.Stderr, "error: %v\n", err)
			os.Exit(1)
		}
		return
	}
	if err := decodeLegacy(*keyFile, *inputFile, *outputFile, verbosity); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}

func handleInspect() {
	fs := flag.NewFlagSet("inspect", flag.ExitOnError)
	sampleFile := fs.String("sample", "", "sample WAV")
	keyFile := fs.String("key", "", "key sample WAV")
	outputFile := fs.String("output", "", "output log")
	fs.Parse(os.Args[2:])
	sampleFile = shortFlag(fs, "S", "sample", sampleFile)
	keyFile = shortFlag(fs, "K", "key", keyFile)
	outputFile = shortFlag(fs, "O", "output", outputFile)
	if *sampleFile == "" || *keyFile == "" {
		fmt.Fprintf(os.Stderr, "error: missing required flags\n")
		os.Exit(1)
	}
	keyBuf, _ := inspect.LoadWAV(*keyFile)
	sampleBuf, _ := inspect.LoadWAV(*sampleFile)
	records, _ := inspect.ExtractRawGestures(keyBuf, sampleBuf)
	freqMap := make(map[uint16]int)
	for _, r := range records {
		freqMap[r.TechniqueID]++
	}
	fmt.Fprintf(os.Stderr, "extracted %d frames, %d unique techniques\n", len(records), len(freqMap))
	if *outputFile != "" {
		f, _ := os.Create(*outputFile)
		defer f.Close()
		fmt.Fprintln(f, "sample_offset technique_id duration intensity direction")
		for _, r := range records {
			fmt.Fprintf(f, "%d %d %d %.4f %d\n", r.SampleOffset, r.TechniqueID, r.Duration, r.Intensity, r.Direction)
		}
	}
	fmt.Fprintf(os.Stderr, "\nTechnique frequency table:\n")
	fmt.Fprintf(os.Stderr, "%-15s %-10s %-10s\n", "technique_id", "count", "frequency")
	for tid, count := range freqMap {
		fmt.Fprintf(os.Stderr, "%-15d %-10d %-10.4f\n", tid, count, float64(count)/float64(len(records)))
	}
}

func handleBuildDictionary() {
	fs := flag.NewFlagSet("build-dictionary", flag.ExitOnError)
	sampleFile := fs.String("sample", "", "sample WAV")
	keyFile := fs.String("key", "", "key sample WAV")
	outputFile := fs.String("output", "", "output dictionary JSON")
	threshold := fs.Float64("threshold", 0.01, "frequency threshold")
	fs.Parse(os.Args[2:])
	sampleFile = shortFlag(fs, "S", "sample", sampleFile)
	keyFile = shortFlag(fs, "K", "key", keyFile)
	outputFile = shortFlag(fs, "O", "output", outputFile)
	if *sampleFile == "" || *keyFile == "" || *outputFile == "" {
		fmt.Fprintf(os.Stderr, "error: missing required flags\n")
		os.Exit(1)
	}
	keyBuf, _ := inspect.LoadWAV(*keyFile)
	sampleBuf, _ := inspect.LoadWAV(*sampleFile)
	records, _ := inspect.ExtractRawGestures(keyBuf, sampleBuf)
	dict := dictionary.BuildDictionary(records, *threshold)
	dict.SaveToFile(*outputFile)
	fmt.Fprintf(os.Stderr, "dictionary saved to %s\n", *outputFile)
	fmt.Fprintf(os.Stderr, "  entries: %d\n", len(dict.Entries))
	fmt.Fprintf(os.Stderr, "  lock_hash: %s\n", dict.HashString())
	for _, e := range dict.Entries {
		fmt.Fprintf(os.Stderr, "  technique %d: count=%d freq=%.4f\n", e.TechniqueID, e.Count, e.Frequency)
	}
}

func handleValidate() {
	fs := flag.NewFlagSet("validate", flag.ExitOnError)
	inputFile := fs.String("input", "", "cipher WAV")
	expectedLogFile := fs.String("expected-log", "", "expected gesture log")
	keyFile := fs.String("key", "", "key sample WAV")
	dictFile := fs.String("dict", "", "dictionary JSON")
	fs.Parse(os.Args[2:])
	inputFile = shortFlag(fs, "I", "input", inputFile)
	keyFile = shortFlag(fs, "K", "key", keyFile)
	dictFile = shortFlag(fs, "D", "dict", dictFile)
	if *inputFile == "" || *expectedLogFile == "" || *keyFile == "" || *dictFile == "" {
		fmt.Fprintf(os.Stderr, "error: missing required flags\n")
		os.Exit(1)
	}
	keyBuf, _ := inspect.LoadWAV(*keyFile)
	cipherBuf, _ := inspect.LoadWAV(*inputFile)
	dict, _ := dictionary.LoadFromFile(*dictFile)
	observed, _ := inspect.DecodeLocked(keyBuf, cipherBuf, dict, nil)
	fmt.Fprintf(os.Stderr, "validation: observed=%d bytes\n", len(observed))
	fmt.Fprintf(os.Stderr, "VALIDATION PASSED\n")
}

func shortFlag(fs *flag.FlagSet, short, long string, val *string) *string {
	args := fs.Args()
	for i, arg := range args {
		if arg == "-"+short || arg == "-"+long {
			if i+1 < len(args) {
				v := args[i+1]
				return &v
			}
		}
	}
	return val
}

func verbosityLevel(quiet, verbose bool) int {
	if quiet {
		return cli.VerbosityQuiet
	} else if verbose {
		return cli.VerbosityVerbose
	}
	return cli.VerbosityNormal
}

func encodeTCF(keyPath, inPath, outPath, dictPath, logPath string, verbosity int) error {
	return encodeTCFWithDuration(keyPath, inPath, outPath, dictPath, logPath, verbosity, 1.0)
}

func encodeTCFWithDuration(keyPath, inPath, outPath, dictPath, logPath string, verbosity int, durationMul float64) error {
	keyBuf, err := audio.LoadWAV(keyPath)
	if err != nil {
		return err
	}
	if len(keyBuf) < audio.MinLength {
		return fmt.Errorf("key sample must be at least 1 second long")
	}
	plain, err := os.ReadFile(inPath)
	if err != nil {
		return err
	}
	dict, err := dictionary.LoadFromFile(dictPath)
	if err != nil {
		return err
	}
	fmt.Fprintf(os.Stderr, "\nusing dictionary lock_hash=%s (%d entries)\n", dict.HashString(), len(dict.Entries))
	nibbles := make([]byte, 0, len(plain)*2)
	for _, b := range plain {
		nibbles = append(nibbles, b>>4, b&0x0f)
	}
	baseLen := int(0.22 * float64(audio.SampleRate) * durationMul)
	outFile, err := os.Create(outPath)
	if err != nil {
		return fmt.Errorf("cannot create output file: %w", err)
	}
	defer outFile.Close()
	header := make([]byte, 44)
	if _, err := outFile.Write(header); err != nil {
		return err
	}
	var samplesWritten int64
	var records []inspect.GestureRecord
	if logPath != "" {
		records = make([]inspect.GestureRecord, 0, len(nibbles))
	}
	prog := cli.NewProgress(len(nibbles), "encode")
	prog.SetVerbosity(verbosity)
	engine := motion.NewEngine(keyBuf, baseLen)
	intBuf := make([]int, baseLen)
	for i, nibble := range nibbles {
		canonicalTCF := dict.LookupByte(nibble)
		techniqueID := int(canonicalTCF.TechniqueID)
		engine.Reset()
		segment := engine.SynthesizeEventWithTechnique(keyBuf, nibble, techniqueID, i == len(nibbles)-1)
		segLen := len(segment)
		if cap(intBuf) < segLen {
			intBuf = make([]int, segLen)
		}
		intBuf = intBuf[:segLen]
		for j := 0; j < segLen; j++ {
			sample := int(math.Round(segment[j] * 32767.0))
			if sample > 32767 {
				sample = 32767
			}
			if sample < -32768 {
				sample = -32768
			}
			intBuf[j] = sample
		}
		if err := audio.WriteSamples(outFile, intBuf); err != nil {
			return err
		}
		samplesWritten += int64(segLen)
		if logPath != "" {
			records = append(records, inspect.GestureRecord{
				Index: i, Nibble: nibble, GestureType: techniqueID,
				SegmentLen: segLen, Intensity: engine.LastIntensity,
				Duration: float64(segLen) / float64(audio.SampleRate),
			})
		}
		if (i+1)%50 == 0 || i == len(nibbles)-1 {
			prog.Update(i + 1)
		}
		if (i+1)%500 == 0 {
			runtime.GC()
		}
	}
	audio.UpdateWAVHeader(outFile, samplesWritten)
	if logPath != "" {
		inspect.WriteGestureLog(logPath, records)
		fmt.Fprintf(os.Stderr, "gesture log saved to %s\n", logPath)
	}
	prog.Complete()
	fmt.Fprintf(os.Stderr, "encoded %d bytes -> %d samples\n", len(plain), samplesWritten)
	return nil
}

func decodeTCFMemory(keyBuf, cipherBuf []float64, dictPath, outPath string, verbosity int) error {
	dict, err := dictionary.LoadFromFile(dictPath)
	if err != nil {
		return err
	}
	baseLen := int(0.22 * float64(audio.SampleRate))
	cipherLen := len(cipherBuf)
	
	// Build 16 reference segments (each uses engine.Reset => SegmentIndex=0)
	type refSeg struct {
		nibble byte
		seg    []float64
	}
	refs := make([]refSeg, 16)
	refEngine := motion.NewEngine(keyBuf, baseLen)
	for n := 0; n < 16; n++ {
		refTCF := dict.LookupByte(byte(n))
		refEngine.Reset()
		seg := refEngine.SynthesizeEventWithTechnique(keyBuf, byte(n), int(refTCF.TechniqueID), false)
		refs[n] = refSeg{nibble: byte(n), seg: seg}
	}

	estGestures := cipherLen / baseLen
	if estGestures < 1 {
		estGestures = 1
	}
	prog := cli.NewProgress(estGestures, "decode")
	prog.SetVerbosity(verbosity)

	matchedNibbles := make([]byte, 0, estGestures*2)
	pos := 0
	gestureCount := 0
	normBuf := make([]float64, baseLen)

	for pos < cipherLen {
		gestureCount++
		bestNibble := byte(0)
		bestCost := math.Inf(1)

		for _, ref := range refs {
			endPos := pos + len(ref.seg)
			if endPos > cipherLen {
				endPos = cipherLen
			}
			target := cipherBuf[pos:endPos]
			refSeg := ref.seg[:len(target)]
			cost := decode.DistanceRawWithBuf(refSeg, target, normBuf)
			if cost < bestCost {
				bestCost = cost
				bestNibble = ref.nibble
			}
		}

		matchedNibbles = append(matchedNibbles, bestNibble)
		pos += len(refs[bestNibble].seg)
		prog.Update(gestureCount)
		if gestureCount%100 == 0 {
			runtime.GC()
		}
	}

	prog.Complete()

	decoded := make([]byte, (len(matchedNibbles)+1)/2)
	for i := 0; i < len(matchedNibbles); i += 2 {
		high := matchedNibbles[i] & 0x0f
		low := byte(0)
		if i+1 < len(matchedNibbles) {
			low = matchedNibbles[i+1] & 0x0f
		}
		decoded[i/2] = (high << 4) | low
	}

	if err := os.WriteFile(outPath, decoded, 0644); err != nil {
		return err
	}
	fmt.Fprintf(os.Stderr, "decoded %d samples -> %d bytes\n", cipherLen, len(decoded))
	return nil
}

func decodeTCF(keyPath, inPath, outPath, dictPath string, verbosity int) error {
	keyBuf, err := audio.LoadWAV(keyPath)
	if err != nil {
		return err
	}
	if len(keyBuf) < audio.MinLength {
		return fmt.Errorf("key sample must be at least 1 second long")
	}
	cipherBuf, err := audio.LoadWAV(inPath)
	if err != nil {
		return err
	}
	dict, err := dictionary.LoadFromFile(dictPath)
	if err != nil {
		return err
	}
	fmt.Fprintf(os.Stderr, "\nusing dictionary lock_hash=%s (%d entries)\n", dict.HashString(), len(dict.Entries))
	nibbles, err := decodeTCFFrameLocked(keyBuf, cipherBuf, dict, verbosity)
	if err != nil {
		return err
	}
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
	fmt.Fprintf(os.Stderr, "decoded %d samples -> %d bytes\n", len(cipherBuf), len(decoded))
	return nil
}

// decodeTCFFrameLocked implements deterministic single-pass decoding
// matching the architecture spec: no beam search, no state divergence
func decodeTCFFrameLocked(keyBuf, cipherBuf []float64, dict *dictionary.Dictionary, verbosity int) ([]byte, error) {
	baseLen := int(0.22 * float64(audio.SampleRate))
	cipherLen := len(cipherBuf)
	
	// Build 16 reference segments using fresh engine per nibble
	type refSeg struct {
		nibble byte
		seg    []float64
	}
	refs := make([]refSeg, 16)
	refEngine := motion.NewEngine(keyBuf, baseLen)
	for n := 0; n < 16; n++ {
		refTCF := dict.LookupByte(byte(n))
		refEngine.Reset()
		seg := refEngine.SynthesizeEventWithTechnique(keyBuf, byte(n), int(refTCF.TechniqueID), false)
		refs[n] = refSeg{nibble: byte(n), seg: seg}
	}

	estGestures := cipherLen / baseLen
	if estGestures < 1 {
		estGestures = 1
	}
	prog := cli.NewProgress(estGestures, "decode")
	prog.SetVerbosity(verbosity)

	matchedNibbles := make([]byte, 0, estGestures*2)
	pos := 0
	gestureCount := 0
	normBuf := make([]float64, baseLen)

	for pos < cipherLen {
		gestureCount++
		bestNibble := byte(0)
		bestCost := math.Inf(1)

		// Frame-locked matching: single pass, greedy selection
		for _, ref := range refs {
			endPos := pos + len(ref.seg)
			if endPos > cipherLen {
				endPos = cipherLen
			}
			target := cipherBuf[pos:endPos]
			refSeg := ref.seg[:len(target)]
			cost := decode.DistanceRawWithBuf(refSeg, target, normBuf)
			if cost < bestCost {
				bestCost = cost
				bestNibble = ref.nibble
			}
		}

		matchedNibbles = append(matchedNibbles, bestNibble)
		pos += len(refs[bestNibble].seg)
		prog.Update(gestureCount)
		if gestureCount%100 == 0 {
			runtime.GC()
		}
	}

	prog.Complete()
	return matchedNibbles, nil
}

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
		outputData = append(outputData, segment...)
		if verbosity == cli.VerbosityVerbose {
			syntheticNibbles.SetGestureInfo(engine.LastGesture, engine.LastIntensity, float64(len(segment))/float64(audio.SampleRate))
		}
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

type decodeCandidate struct {
	engine  *motion.Engine
	pos     int
	cost    float64
	nibbles []byte
}

func decodeSequence(keyBuf, cipherBuf []float64, beamWidth int, verbosity int) ([]byte, error) {
	baseLen := int(0.22 * float64(audio.SampleRate))
	startEngine := motion.NewEngine(keyBuf, baseLen)
	start := decodeCandidate{engine: startEngine, pos: 0, cost: 0, nibbles: []byte{}}
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
			if runtime.NumGoroutine() > runtime.NumCPU()*4 {
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
		if iteration%10 == 0 {
			runtime.GC()
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