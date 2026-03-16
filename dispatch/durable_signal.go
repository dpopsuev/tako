package dispatch

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"sync"
)

// DurableSignalBus wraps a SignalBus with persistent tee-write to a
// JSON-Lines file. On crash recovery, Replay() re-reads the file and
// populates the in-memory bus with historical signals.
type DurableSignalBus struct {
	*SignalBus
	mu   sync.Mutex
	path string
	file *os.File
	enc  *json.Encoder
}

// NewDurableSignalBus creates a durable bus that persists signals to
// the given file path. The file is created if it doesn't exist, or
// opened for append if it does.
func NewDurableSignalBus(path string) (*DurableSignalBus, error) {
	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		return nil, fmt.Errorf("open signal log %s: %w", path, err)
	}
	return &DurableSignalBus{
		SignalBus: NewSignalBus(),
		path:     path,
		file:     f,
		enc:      json.NewEncoder(f),
	}, nil
}

// Emit appends a signal to both the in-memory bus and the persistent log.
func (d *DurableSignalBus) Emit(event, agent, caseID, step string, meta map[string]string) {
	d.SignalBus.Emit(event, agent, caseID, step, meta)

	d.mu.Lock()
	defer d.mu.Unlock()
	if d.enc != nil {
		signals := d.SignalBus.Since(d.SignalBus.Len() - 1)
		if len(signals) > 0 {
			_ = d.enc.Encode(signals[0])
		}
	}
}

// Replay reads persisted signals from the file and populates the
// in-memory bus. Call this once on startup before any new Emit calls
// to restore state after a crash.
func (d *DurableSignalBus) Replay() (int, error) {
	f, err := os.Open(d.path)
	if err != nil {
		if os.IsNotExist(err) {
			return 0, nil
		}
		return 0, fmt.Errorf("replay signal log: %w", err)
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	count := 0
	for scanner.Scan() {
		var sig Signal
		if err := json.Unmarshal(scanner.Bytes(), &sig); err != nil {
			continue
		}
		d.SignalBus.Emit(sig.Event, sig.Agent, sig.CaseID, sig.Step, sig.Meta)
		count++
	}
	return count, scanner.Err()
}

// Close flushes and closes the persistent log file.
func (d *DurableSignalBus) Close() error {
	d.mu.Lock()
	defer d.mu.Unlock()
	if d.file != nil {
		err := d.file.Close()
		d.file = nil
		d.enc = nil
		return err
	}
	return nil
}

// Path returns the file path of the persistent log.
func (d *DurableSignalBus) Path() string {
	return d.path
}
