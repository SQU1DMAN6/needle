package cli

import (
	"fmt"
)

// Progress tracks and displays encoding/decoding progress
type Progress struct {
	current int
	total   int
	stage   string
}

// NewProgress creates a new progress tracker
func NewProgress(total int, stage string) *Progress {
	return &Progress{
		current: 0,
		total:   total,
		stage:   stage,
	}
}

// Update increments progress and prints status
func (p *Progress) Update(current int) {
	p.current = current
	p.Print()
}

// Print outputs current progress
func (p *Progress) Print() {
	if p.total == 0 {
		fmt.Printf("[%s] starting...\n", p.stage)
		return
	}

	pct := int(100 * p.current / p.total)
	fmt.Printf("[%s] %d/%d (%d%%)\n", p.stage, p.current, p.total, pct)
}

// Complete marks progress as done
func (p *Progress) Complete() {
	fmt.Printf("[%s] complete\n", p.stage)
}
