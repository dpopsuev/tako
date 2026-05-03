package ergograph

import (
	"fmt"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/dpopsuev/tako/store"
)

func TestDoltLedgerConcurrentAppend(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "racedb")
	db, err := store.Open(dir)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer db.Close()
	if err := db.Migrate(); err != nil {
		t.Fatalf("migrate: %v", err)
	}

	pool := NewDoltLedger(db.DB)

	var wg sync.WaitGroup
	for i := range 5 {
		wg.Add(1)
		go func(n int) {
			defer wg.Done()
			for j := range 10 {
				err := pool.Append(Record{
					Identity:  fmt.Sprintf("worker-%d", n),
					Action:    fmt.Sprintf("action-%d", j),
					Timestamp: time.Now(),
					Labels:    map[string]string{"goroutine": fmt.Sprintf("%d", n)},
				})
				if err != nil {
					t.Errorf("append %d-%d: %v", n, j, err)
				}
			}
		}(i)
	}
	wg.Wait()

	if pool.Len() != 50 {
		t.Errorf("expected 50 records, got %d", pool.Len())
	}

	if err := pool.VerifyChain(); err != nil {
		t.Errorf("chain broken after concurrent append: %v", err)
	}
}
