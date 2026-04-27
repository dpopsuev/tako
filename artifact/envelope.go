package artifact

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"sort"
	"time"
)

// Envelope wraps a work artifact as it flows through the Fab graph.
type Envelope struct {
	ID        string
	Origin    string
	Payload   []byte
	CreatedAt time.Time
	Labels    map[string]string
	Hash      string
}

// NewEnvelope creates an Envelope with the given origin and payload.
func NewEnvelope(origin string, payload []byte) Envelope {
	return Envelope{
		Origin:    origin,
		Payload:   payload,
		CreatedAt: time.Now(),
		Labels:    make(map[string]string),
	}
}

// Seal computes the hash over payload + labels. Called by Contract after stamping.
func (e *Envelope) Seal() {
	h := sha256.New()
	h.Write(e.Payload)
	keys := make([]string, 0, len(e.Labels))
	for k := range e.Labels {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		fmt.Fprintf(h, "%s=%s", k, e.Labels[k])
	}
	e.Hash = hex.EncodeToString(h.Sum(nil))
}

// Verify checks that the hash matches payload + labels.
func (e Envelope) Verify() bool {
	if e.Hash == "" {
		return false
	}
	h := sha256.New()
	h.Write(e.Payload)
	keys := make([]string, 0, len(e.Labels))
	for k := range e.Labels {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		fmt.Fprintf(h, "%s=%s", k, e.Labels[k])
	}
	return e.Hash == hex.EncodeToString(h.Sum(nil))
}
