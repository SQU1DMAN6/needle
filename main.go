package main

import (
	"flag"
	"fmt"
	"math"
	"os"

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
		fmt.Println("needle version 1.0.0")
	case "help", "-h", "--help":
		printUsage()
	default:
		fmt.Fprintf(os.Stderr, "unknown command: %s\n", command)
		printUsage()
		os.Exit(1)
	}
}

func printUsage() {
	fmt.Fprintf(os.Stderr, `Needle - Key-Conditioned Audio Gesture Cipher v2.0.0

Usage:
  needle encode  -key sample.wav -input plaintext.txt -output cipher.wav
  needle decode  -key sample.wav -input cipher.wav -output plaintext.txt
  needle version
  needle help

Encode:
  Encode plaintext into cipher audio using a key sample.
  -key:    path to key sample WAV file (required)
  -input:  path to plaintext file to encode (required)
  -output: path to output cipher WAV file (required)

Decode:
  Decode cipher audio back into plaintext using the original key sample.
  -key:    path to key sample WAV file (required)
  -input:  path to cipher WAV file to decode (required)
  -output: path to output plaintext file (required)

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

	fs.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage: needle encode -key sample.wav -input plaintext.txt -output cipher.wav\n")
	}

	fs.Parse(os.Args[2:])

	if *keyFile == "" || *inputFile == "" || *outputFile == "" {
		fmt.Fprintf(os.Stderr, "error: missing required flags\n")
		fmt.Fprintf(os.Stderr, "Usage: needle encode -key sample.wav -input plaintext.txt -output cipher.wav\n")
		os.Exit(1)
	}

	if err := encodeFile(*keyFile, *inputFile, *outputFile); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}

func handleDecode() {
	fs := flag.NewFlagSet("decode", flag.ExitOnError)
	keyFile := fs.String("key", "", "key sample WAV file")
	inputFile := fs.String("input", "", "cipher WAV file to decode")
	outputFile := fs.String("output", "", "output plaintext file")

	fs.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage: needle decode -key sample.wav -input cipher.wav -output plaintext.txt\n")
	}

	fs.Parse(os.Args[2:])

	if *keyFile == "" || *inputFile == "" || *outputFile == "" {
		fmt.Fprintf(os.Stderr, "error: missing required flags\n")
		fmt.Fprintf(os.Stderr, "Usage: needle decode -key sample.wav -input cipher.wav -output plaintext.txt\n")
		os.Exit(1)
	}

	if err := decodeFile(*keyFile, *inputFile, *outputFile); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}

func encodeFile(keyPath, inPath, outPath string) error {
	prog := cli.NewProgress(5, "encode")

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

	segmentLen := int(0.25 * float64(audio.SampleRate))
	outputData := make([]float64, len(nibbles)*segmentLen)

	prog.Update(3)
	prog.Print()
	engine := motion.NewEngine(keyBuf, segmentLen)
	for i, nibble := range nibbles {
		engine.Reset()
		segment := engine.SynthesizeSegment(keyBuf, nibble, 0) // Use position 0 for deterministic reversibility
		copy(outputData[i*segmentLen:(i+1)*segmentLen], segment)
		if (i+1)%20 == 0 {
			pct := int(100 * (i + 1) / len(nibbles))
			fmt.Printf("[encode] synthesizing %d/%d nibbles (%d%%)\n", i+1, len(nibbles), pct)
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

func decodeFile(keyPath, inPath, outPath string) error {
	prog := cli.NewProgress(5, "decode")

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

	segmentLen := int(0.25 * float64(audio.SampleRate))
	if len(cipherBuf)%segmentLen != 0 {
		return fmt.Errorf("invalid cipher length: not a multiple of segment size")
	}

	prog.Update(3)
	prog.Print()

	// Build feature library with fresh engine for each nibble
	library := make([]decode.Features, 16)
	for n := 0; n < 16; n++ {
		freshEngine := motion.NewEngine(keyBuf, segmentLen)
		segment := freshEngine.SynthesizeSegment(keyBuf, byte(n), 0)
		library[n] = decode.ExtractFeatures(segment)
	}

	prog.Update(4)
	prog.Print()
	nSegments := len(cipherBuf) / segmentLen
	nibbles := make([]byte, nSegments)
	for i := 0; i < nSegments; i++ {
		segment := cipherBuf[i*segmentLen : (i+1)*segmentLen]
		best := byte(0)
		bestDist := math.Inf(1)

		features := decode.ExtractFeatures(segment)
		for n := 0; n < 16; n++ {
			d := decode.Distance(features, library[n])
			if d < bestDist {
				bestDist = d
				best = byte(n)
			}
		}
		nibbles[i] = best

		if (i+1)%20 == 0 {
			pct := int(100 * (i + 1) / nSegments)
			fmt.Printf("[decode] classifying %d/%d segments (%d%%)\n", i+1, nSegments, pct)
		}
	}

	// Convert nibbles back to bytes
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
