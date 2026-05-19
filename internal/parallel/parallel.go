package parallel

import (
	"needle/internal/decode"
	"needle/internal/motion"
	"sync"
)

// EncodeParallel encodes nibbles in parallel using worker goroutines
func EncodeParallel(engine *motion.Engine, source []float64, nibbles []byte, segmentLen int, numWorkers int) []float64 {
	nNibbles := len(nibbles)
	outputData := make([]float64, nNibbles*segmentLen)

	// Channel for work distribution
	type job struct {
		index  int
		nibble byte
	}
	jobs := make(chan job, nNibbles)
	var wg sync.WaitGroup

	// Spawn workers
	for w := 0; w < numWorkers; w++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			workerEngine := motion.NewEngine(source, segmentLen)
			for job := range jobs {
				workerEngine.Reset()
				segment := workerEngine.SynthesizeEvent(source, job.nibble, false)
				copy(outputData[job.index*segmentLen:(job.index+1)*segmentLen], segment)
			}
		}()
	}

	// Distribute work
	go func() {
		for i, nibble := range nibbles {
			jobs <- job{index: i, nibble: nibble}
		}
		close(jobs)
	}()

	wg.Wait()
	return outputData
}

// DecodeParallel builds library and decodes in parallel
func DecodeParallel(keyBuf []float64, cipherBuf []float64, segmentLen int, numWorkers int) []byte {
	nSegments := len(cipherBuf) / segmentLen

	// Build library in parallel
	type libJob struct {
		nibble byte
		result chan decode.Features
	}
	libraryJobs := make(chan libJob, 16)
	library := make([]decode.Features, 16)
	var libWg sync.WaitGroup

	// Library building workers
	for w := 0; w < numWorkers; w++ {
		libWg.Add(1)
		go func() {
			defer libWg.Done()
			for job := range libraryJobs {
				freshEngine := motion.NewEngine(keyBuf, segmentLen)
				segment := freshEngine.SynthesizeEvent(keyBuf, job.nibble, false)
				job.result <- decode.ExtractFeatures(segment)
			}
		}()
	}

	// Submit library jobs
	go func() {
		for n := 0; n < 16; n++ {
			result := make(chan decode.Features, 1)
			libraryJobs <- libJob{nibble: byte(n), result: result}
			go func(n int, ch chan decode.Features) {
				library[n] = <-ch
			}(n, result)
		}
		close(libraryJobs)
	}()

	libWg.Wait()

	// Decode segments in parallel
	type decodeJob struct {
		index   int
		segment []float64
	}
	decodeJobs := make(chan decodeJob, nSegments)
	nibbles := make([]byte, nSegments)
	var decWg sync.WaitGroup

	for w := 0; w < numWorkers; w++ {
		decWg.Add(1)
		go func() {
			defer decWg.Done()
			for job := range decodeJobs {
				best := byte(0)
				bestDist := 1e9
				features := decode.ExtractFeatures(job.segment)
				for n := 0; n < 16; n++ {
					d := decode.Distance(features, library[n])
					if d < bestDist {
						bestDist = d
						best = byte(n)
					}
				}
				nibbles[job.index] = best
			}
		}()
	}

	go func() {
		for i := 0; i < nSegments; i++ {
			segment := make([]float64, segmentLen)
			copy(segment, cipherBuf[i*segmentLen:(i+1)*segmentLen])
			decodeJobs <- decodeJob{index: i, segment: segment}
		}
		close(decodeJobs)
	}()

	decWg.Wait()
	return nibbles
}
