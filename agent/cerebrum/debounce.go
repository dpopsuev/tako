package cerebrum

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
)

type callFingerprint struct {
	Name     string
	ArgsHash [32]byte
}

type Debouncer struct {
	window    int
	threshold int
	recent    []callFingerprint
}

func NewDebouncer(window, threshold int) *Debouncer {
	if window <= 0 {
		window = 3
	}
	if threshold <= 0 {
		threshold = 2
	}
	return &Debouncer{window: window, threshold: threshold}
}

func (d *Debouncer) Check(name string, args json.RawMessage) bool {
	fp := callFingerprint{
		Name:     name,
		ArgsHash: sha256.Sum256(args),
	}

	count := 0
	for _, r := range d.recent {
		if r == fp {
			count++
		}
	}

	d.recent = append(d.recent, fp)
	if len(d.recent) > d.window {
		d.recent = d.recent[len(d.recent)-d.window:]
	}

	return count >= d.threshold
}

func (d *Debouncer) BlockedMessage(name string) string {
	return fmt.Sprintf("BLOCKED: duplicate call to %s — try a different approach", name)
}

func (d *Debouncer) Reset() {
	d.recent = d.recent[:0]
}
