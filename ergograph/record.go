package ergograph

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"time"
)

// Record is a single entry in the Ergograph — one unit of recorded work.
type Record struct {
	Identity  string
	Action    string
	Timestamp time.Time
	Sequence  uint64
	Labels    map[string]string
	Payload   []byte
	Hash      string
	PrevHash  string
}

// ComputeHash calculates the SHA-256 hash chain link for this Record.
func (r *Record) ComputeHash() {
	data := fmt.Sprintf("%s|%s|%d|%d|%s|%x",
		r.Identity, r.Action, r.Timestamp.UnixNano(), r.Sequence, r.PrevHash, r.Payload)
	sum := sha256.Sum256([]byte(data))
	r.Hash = hex.EncodeToString(sum[:])
}

// OAE is Overall Agent Effectiveness: Availability x Performance x Quality.
type OAE struct {
	Availability float64
	Performance  float64
	Quality      float64
}

// Score returns the composite OAE value.
func (o OAE) Score() float64 {
	return o.Availability * o.Performance * o.Quality
}
