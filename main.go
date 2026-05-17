package main

import (
	"flag"
	"fmt"
	"os"
	"runtime"
	"sort"

	"needle/internal/audio"
	"needle/internal/cli"
	"needle/internal/decode"
	"needle/internal/motion"
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
	case "version":
		fmt.Println("needle version 1.5.0")
	case "help", "-h", "--help":
		printUsage()
	default:
		fmt.Fprintf(os.Stderr, "unknown command: %s\n", command)
		printUsage()
		os.Exit(1)
	}
}

func printUsage() {
	fmt.Fprintf(os.Stderr, `Needle - Key-Conditioned Audio Gesture Cipher v1.5.0

Usage:
  needle encode  -key sample.wav -input plaintext.txt -output cipher.wav [-qq]
  needle decode  -key sample.wav -input cipher.wav -output plaintext.txt [-qq]
  needle version
  needle help

Encode:
  Encode plaintext into cipher audio using a key sample.
  -key:    path to key sample WAV file (required)
  -input:  path to plaintext file to encode (required)
  -output: path to output cipher WAV file (required)
  -q:      quiet mode (suppress progress output)
  -qq:     verbose mode (show gesture details, physics state)

Decode:
  Decode cipher audio back into plaintext using the original key sample.
  -key:    path to key sample WAV file (required)
  -input:  path to cipher WAV file to decode (required)
  -output: path to output plaintext file (required)
  -q:      quiet mode
  -qq:     verbose mode

Requirements:
  - Mono WAV files at 44100 Hz, 16-bit
  - Key sample must be at least 1 second long
`)
}

func handleEncode() {
	fs := flag.NewFlagSet("encode", flag.ExitOnError)
	keyFile := fs.String("key", "", "key sample WAV file")
	inputFile := fs.String("input", "", "plaintext file to encode")
	outputFile := fs.String("output", "", "output cipher WAV file")
	threads := fs.Int("threads", runtime.NumCPU(), "number of parallel workers")
	quiet := fs.Bool("q", false, "quiet mode")
	verbose := fs.Bool("qq", false, "verbose mode (detailed progress)")

	fs.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage: needle encode -key sample.wav -input plaintext.txt -output cipher.wav [-threads N] [-q|-qq]\n")
	}

	fs.Parse(os.Args[2:])

	if *keyFile == "" || *inputFile == "" || *outputFile == "" {
		fmt.Fprintf(os.Stderr, "error: missing required flags\n")
		fmt.Fprintf(os.Stderr, "Usage: needle encode -key sample.wav -input plaintext.txt -output cipher.wav [-threads N]\n")
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

	if err := encodeFile(*keyFile, *inputFile, *outputFile, verbosity); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}

func handleDecode() {
	fs := flag.NewFlagSet("decode", flag.ExitOnError)
	keyFile := fs.String("key", "", "key sample WAV file")
	inputFile := fs.String("input", "", "cipher WAV file to decode")
	outputFile := fs.String("output", "", "output plaintext file")
	threads := fs.Int("threads", runtime.NumCPU(), "number of parallel workers")
	quiet := fs.Bool("q", false, "quiet mode")
	verbose := fs.Bool("qq", false, "verbose mode (detailed progress)")

	fs.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage: needle decode -key sample.wav -input cipher.wav -output plaintext.txt [-threads N] [-q|-qq]\n")
	}

	fs.Parse(os.Args[2:])

	if *keyFile == "" || *inputFile == "" || *outputFile == "" {
		fmt.Fprintf(os.Stderr, "error: missing required flags\n")
		fmt.Fprintf(os.Stderr, "Usage: needle decode -key sample.wav -input cipher.wav -output plaintext.txt [-threads N]\n")
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

	if err := decodeFile(*keyFile, *inputFile, *outputFile, verbosity); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}

func encodeFile(keyPath, inPath, outPath string, verbosity int) error {
	prog := cli.NewProgress(5, "encode")
	prog.SetVerbosity(verbosity)

	prog.Update(1)
	prog.Print()
	keyBuf, err := audio.LoadWAV(keyPath)
	if err != nil {
		return err
	}

	if len(keyBuf) < audio.MinLength {
		return fmt.Errorf("key sample must be at least 1 second long")
	}

	prog.Update(2)
	prog.Print()
	plain, err := os.ReadFile(inPath)
	if err != nil {
		return err
	}

	// Encode each byte as two nibbles (4-bit values)
	nibbles := make([]byte, 0, len(plain)*2)
	for _, b := range plain {
		nibbles = append(nibbles, b>>4, b&0x0f)
	}

	baseLen := int(0.22 * float64(audio.SampleRate))
	outputData := make([]float64, 0, len(nibbles)*baseLen)

	prog.Update(3)
	prog.Print()

	engine := motion.NewEngine(keyBuf, baseLen)
	syntheticNibbles := cli.NewProgress(len(nibbles), "synthesize")
	syntheticNibbles.SetVerbosity(verbosity)

	for i, nibble := range nibbles {
		segment := engine.SynthesizeEvent(keyBuf, nibble)
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
			syntheticNibbles.Print()
		}
	}

	prog.Update(4)
	prog.Print()
	if err := audio.SaveWAV(outPath, outputData); err != nil {
		return err
	}

	prog.Update(5)
	prog.Complete()
	return nil
}

func decodeFile(keyPath, inPath, outPath string, verbosity int) error {
	prog := cli.NewProgress(5, "decode")
	prog.SetVerbosity(verbosity)

	prog.Update(1)
	prog.Print()
	keyBuf, err := audio.LoadWAV(keyPath)
	if err != nil {
		return err
	}

	if len(keyBuf) < audio.MinLength {
		return fmt.Errorf("key sample must be at least 1 second long")
	}

	prog.Update(2)
	prog.Print()
	cipherBuf, err := audio.LoadWAV(inPath)
	if err != nil {
		return err
	}

	prog.Update(3)
	prog.Print()

	nibbles, err := decodeSequence(keyBuf, cipherBuf, 8, verbosity)
	if err != nil {
		return err
	}

	prog.Update(4)
	prog.Print()

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
	start := decodeCandidate{
		engine:  motion.NewEngine(keyBuf, baseLen),
		pos:     0,
		cost:    0,
		nibbles: []byte{},
	}

	beam := []decodeCandidate{start}
	complete := make([]decodeCandidate, 0)
	targetLen := len(cipherBuf)
	iteration := 0

	decodeProg := cli.NewProgress(targetLen/baseLen, "beam_search")
	decodeProg.SetVerbosity(verbosity)

	for len(beam) > 0 {
		nextBeam := make([]decodeCandidate, 0, len(beam)*16)
		iteration++

		for _, candidate := range beam {
			if candidate.pos == targetLen {
				complete = append(complete, candidate)
				continue
			}

			for n := 0; n < 16; n++ {
				branch := decodeCandidate{
					engine:  candidate.engine.Clone(),
					pos:     candidate.pos,
					cost:    candidate.cost,
					nibbles: append([]byte(nil), candidate.nibbles...),
				}

				segment := branch.engine.SynthesizeEvent(keyBuf, byte(n))
				length := len(segment)
				if branch.pos+length > targetLen {
					continue
				}

				target := cipherBuf[branch.pos : branch.pos+length]
				distance := decode.Distance(decode.ExtractFeatures(segment), decode.ExtractFeatures(target))
				branch.cost += distance
				branch.pos += length
				branch.nibbles = append(branch.nibbles, byte(n))
				nextBeam = append(nextBeam, branch)
			}
		}

		if len(nextBeam) == 0 {
			break
		}

		beam = pruneCandidates(nextBeam, beamWidth)

		// Report decode progress
		if len(beam) > 0 {
			decodeProg.Update(beam[0].pos / baseLen)
			decodeProg.SetCostInfo(beam[0].cost, len(beam), beam[0].pos)
			if iteration%5 == 0 {
				decodeProg.Print()
			}
		}
	}

	if len(complete) == 0 {
		return nil, fmt.Errorf("failed to decode cipher: no complete path found")
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
