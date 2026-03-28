package view

import (
	"sync"
	"time"
)

// TxRxDirection indicates whether a TX/RX entry is an outgoing prompt or incoming response.
type TxRxDirection string

const (
	TxDirection TxRxDirection = "tx"
	RxDirection TxRxDirection = "rx"
)

// TxRxEntry represents a single prompt (TX) or response (RX) captured during a walk.
type TxRxEntry struct {
	Timestamp   time.Time     `json:"timestamp"`
	Walker      string        `json:"walker"`
	Direction   TxRxDirection `json:"direction"`
	Node        string        `json:"node"`
	ContentType string        `json:"content_type,omitempty"`
	Content     string        `json:"content"`
	Truncated   bool          `json:"truncated,omitempty"`
}

// TxRxLog is a thread-safe ring buffer of TX/RX entries.
type TxRxLog struct {
	mu      sync.RWMutex
	entries []TxRxEntry
	head    int
	count   int
	cap     int
}

// NewTxRxLog creates a ring buffer with the given capacity.
func NewTxRxLog(capacity int) *TxRxLog {
	return &TxRxLog{
		entries: make([]TxRxEntry, capacity),
		cap:     capacity,
	}
}

// Push adds an entry, overwriting the oldest if full.
func (l *TxRxLog) Push(e *TxRxEntry) {
	l.mu.Lock()
	defer l.mu.Unlock()
	idx := (l.head + l.count) % l.cap
	if l.count == l.cap {
		l.head = (l.head + 1) % l.cap
	} else {
		l.count++
	}
	l.entries[idx] = *e
}

// Len returns the number of entries.
func (l *TxRxLog) Len() int {
	l.mu.RLock()
	defer l.mu.RUnlock()
	return l.count
}

// All returns all entries in order (oldest first).
func (l *TxRxLog) All() []TxRxEntry {
	l.mu.RLock()
	defer l.mu.RUnlock()
	out := make([]TxRxEntry, l.count)
	for i := 0; i < l.count; i++ {
		out[i] = l.entries[(l.head+i)%l.cap]
	}
	return out
}

// ForWalker returns entries for a specific walker. Empty string returns all.
func (l *TxRxLog) ForWalker(walkerID string) []TxRxEntry {
	if walkerID == "" {
		return l.All()
	}
	l.mu.RLock()
	defer l.mu.RUnlock()
	var out []TxRxEntry
	for i := 0; i < l.count; i++ {
		e := l.entries[(l.head+i)%l.cap]
		if e.Walker == walkerID {
			out = append(out, e)
		}
	}
	return out
}

// LastTxRx returns the last TX and last RX entries for the given walker.
// Returns nil for either if not found.
func (l *TxRxLog) LastTxRx(walkerID string) (tx, rx *TxRxEntry) {
	l.mu.RLock()
	defer l.mu.RUnlock()
	for i := l.count - 1; i >= 0; i-- {
		e := l.entries[(l.head+i)%l.cap]
		if walkerID != "" && e.Walker != walkerID {
			continue
		}
		if e.Direction == TxDirection && tx == nil {
			eCopy := e
			tx = &eCopy
		}
		if e.Direction == RxDirection && rx == nil {
			eCopy := e
			rx = &eCopy
		}
		if tx != nil && rx != nil {
			break
		}
	}
	return
}
