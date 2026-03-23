package kami

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"time"
)

// Recorder subscribes to an EventBridge and writes timestamped JSONL.
// Each line is a JSON-encoded Event. Close flushes and closes the writer.
type Recorder struct {
	bridge *EventBridge
	subID  int
	ch     <-chan Event
	writer io.WriteCloser
	done   chan struct{}
}

// NewRecorder creates a recorder that writes to the given file path.
func NewRecorder(bridge *EventBridge, path string) (*Recorder, error) {
	f, err := os.Create(path)
	if err != nil {
		return nil, fmt.Errorf("create recording: %w", err)
	}
	return NewRecorderWriter(bridge, f), nil
}

// NewRecorderWriter creates a recorder that writes to any WriteCloser.
func NewRecorderWriter(bridge *EventBridge, w io.WriteCloser) *Recorder {
	id, ch := bridge.Subscribe()
	return &Recorder{
		bridge: bridge,
		subID:  id,
		ch:     ch,
		writer: w,
		done:   make(chan struct{}),
	}
}

// Start begins recording in a background goroutine.
// Returns when Close is called.
func (r *Recorder) Start() {
	go func() {
		defer close(r.done)
		enc := json.NewEncoder(r.writer)
		for evt := range r.ch {
			_ = enc.Encode(evt)
		}
	}()
}

// Close unsubscribes from the bridge, flushes, and closes the writer.
func (r *Recorder) Close() error {
	r.bridge.Unsubscribe(r.subID)
	<-r.done
	return r.writer.Close()
}

// Replayer reads a JSONL recording and emits events to an EventBridge
// with the original timing (scaled by Speed).
type Replayer struct {
	bridge *EventBridge
	reader io.ReadCloser
	Speed  float64 // multiplier: 2.0 = 2x speed, 0.5 = half speed
}

// NewReplayer creates a replayer from a file path.
func NewReplayer(bridge *EventBridge, path string, speed float64) (*Replayer, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("open recording: %w", err)
	}
	return NewReplayerReader(bridge, f, speed), nil
}

// NewReplayerReader creates a replayer from any ReadCloser.
func NewReplayerReader(bridge *EventBridge, r io.ReadCloser, speed float64) *Replayer {
	if speed <= 0 {
		speed = 1.0
	}
	return &Replayer{
		bridge: bridge,
		reader: r,
		Speed:  speed,
	}
}

// Play replays all events, respecting inter-event timing scaled by Speed.
// Blocks until all events are emitted or the done channel is closed.
func (rp *Replayer) Play(done <-chan struct{}) error {
	defer rp.reader.Close()

	scanner := bufio.NewScanner(rp.reader)

	var prev time.Time
	for scanner.Scan() {
		var evt Event
		if err := json.Unmarshal(scanner.Bytes(), &evt); err != nil {
			continue
		}

		if !prev.IsZero() && !evt.Timestamp.IsZero() {
			gap := evt.Timestamp.Sub(prev)
			if gap > 0 {
				scaledGap := time.Duration(float64(gap) / rp.Speed)
				select {
				case <-done:
					return nil
				case <-time.After(scaledGap):
				}
			}
		}
		prev = evt.Timestamp

		rp.bridge.Emit(evt)
	}
	return scanner.Err()
}
