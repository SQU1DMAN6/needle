package cli

import (
	"fmt"
	"time"
)

// Verbosity levels
const (
	VerbosityQuiet   = -1 // -q: minimal output
	VerbosityNormal  = 0  // default: standard progress
	VerbosityVerbose = 1  // -qq: detailed output
	VerbosityDebug   = 2  // -qqq: full debug info
)

// Progress tracks and displays encoding/decoding progress
type Progress struct {
	current     int
	total       int
	stage       string
	verbosity   int
	startTime   time.Time
	itemsPerSec float64
	lastUpdate  time.Time
	gestureInfo string
	physicsInfo string
	costInfo    string
	firstPrint  bool
}

// NewProgress creates a new progress tracker
func NewProgress(total int, stage string) *Progress {
	return &Progress{
		current:    0,
		total:      total,
		stage:      stage,
		verbosity:  VerbosityNormal,
		startTime:  time.Now(),
		lastUpdate: time.Now(),
		firstPrint: true,
	}
}

// SetVerbosity sets the output verbosity level
func (p *Progress) SetVerbosity(v int) {
	p.verbosity = v
}

// SetGestureInfo sets detailed gesture information for display
func (p *Progress) SetGestureInfo(gestureType int, intensity float64, duration float64) {
	p.gestureInfo = fmt.Sprintf("gesture=%d intensity=%.2f duration=%.0fms", gestureType, intensity, duration*1000)
}

// SetPhysicsInfo sets physics state information
func (p *Progress) SetPhysicsInfo(platterVel, stylusDrag, crossfader float64) {
	p.physicsInfo = fmt.Sprintf("platter=%.3f stylus=%.3f crossfader=%.2f", platterVel, stylusDrag, crossfader)
}

// SetCostInfo sets decoding cost information
func (p *Progress) SetCostInfo(cost float64, beamWidth int, position int) {
	p.costInfo = fmt.Sprintf("cost=%.4f beam=%d pos=%d", cost, beamWidth, position)
}

// Update increments progress and prints status
func (p *Progress) Update(current int) {
	p.current = current
	now := time.Now()
	elapsed := now.Sub(p.lastUpdate).Seconds()
	if elapsed > 0 && current > 0 {
		p.itemsPerSec = float64(current) / now.Sub(p.startTime).Seconds()
	}
	p.lastUpdate = now
	p.Print()
}

// Print outputs current progress based on verbosity level
func (p *Progress) Print() {
	if p.verbosity == VerbosityQuiet {
		return
	}

	if p.total == 0 {
		fmt.Printf("[%s] starting...\n", p.stage)
		return
	}

	pct := int(100 * p.current / p.total)
	elapsed := time.Since(p.startTime).Seconds()

	// Normal verbosity: compact single-line progress with ETA
	if p.verbosity == VerbosityNormal {
		remaining := 0.0
		if p.itemsPerSec > 0 && p.current < p.total {
			remaining = float64(p.total-p.current) / p.itemsPerSec
		}

		// progress bar
		barWidth := 30
		filled := int(float64(barWidth) * float64(p.current) / float64(p.total))
		if filled > barWidth {
			filled = barWidth
		}
		bar := ""
		for i := 0; i < filled; i++ {
			bar += "#"
		}
		for i := filled; i < barWidth; i++ {
			bar += "-"
		}

		fmt.Printf("\r[%s] [%s] %d/%d (%d%%) %.1f/s ETA %.0fs",
			p.stage, bar, p.current, p.total, pct, p.itemsPerSec, remaining)
		if p.firstPrint {
			p.firstPrint = false
		}
		if p.current >= p.total {
			fmt.Println("")
		}
		return
	}

	// Verbose: detailed output
	if p.verbosity >= VerbosityVerbose {
		fmt.Printf("[%s] %d/%d (%d%%) | elapsed=%.1fs rate=%.1f/s\n",
			p.stage, p.current, p.total, pct, elapsed, p.itemsPerSec)

		if p.gestureInfo != "" {
			fmt.Printf("  └─ %s\n", p.gestureInfo)
		}
		if p.physicsInfo != "" {
			fmt.Printf("  └─ physics: %s\n", p.physicsInfo)
		}
		if p.costInfo != "" {
			fmt.Printf("  └─ decode: %s\n", p.costInfo)
		}
	}
}

// Complete marks progress as done
func (p *Progress) Complete() {
	if p.verbosity == VerbosityQuiet {
		return
	}

	elapsed := time.Since(p.startTime)
	rate := 0.0
	if elapsed.Seconds() > 0 {
		rate = float64(p.current) / elapsed.Seconds()
	}

	if p.verbosity == VerbosityNormal {
		fmt.Printf("[%s] complete (%.1f/s, %.1fs)\n", p.stage, rate, elapsed.Seconds())
	} else {
		fmt.Printf("[%s] ✓ complete | processed=%d rate=%.1f/s total_time=%.1fs\n",
			p.stage, p.current, rate, elapsed.Seconds())
	}
}
