package main

import (
	"math"
	"os"

	"needle/internal/audio"
)

func main() {
	// create three sample WAVs in parent directory
	makeSine("../sample.wav", 440.0)
	makeSine("../sample2.wav", 220.0)
	makeSine("../sample3.wav", 880.0)
}

func makeSine(path string, freq float64) {
	d := 1.0 // seconds
	samples := int(float64(audio.SampleRate) * d)
	buf := make([]float64, samples)
	for i := 0; i < samples; i++ {
		t := float64(i) / float64(audio.SampleRate)
		buf[i] = 0.2 * math.Sin(2*math.Pi*freq*t)
	}
	_ = os.MkdirAll("..", 0755)
	audio.SaveWAV(path, buf)
}
