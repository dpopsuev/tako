package depo

import (
	"fmt"
	"path/filepath"
	"sync"
	"testing"

	"github.com/dpopsuev/tako/artifact"
	"github.com/dpopsuev/tako/store"
)

func TestDoltShelfConcurrentPush(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "racedb")
	db, err := store.Open(dir)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer db.Close()
	if err := db.Migrate(); err != nil {
		t.Fatalf("migrate: %v", err)
	}

	dp := NewDoltDepo(db.DB, "race")
	shelf := dp.Shelf("intake")

	var wg sync.WaitGroup
	for i := range 5 {
		wg.Add(1)
		go func(n int) {
			defer wg.Done()
			for j := range 10 {
				env := artifact.NewEnvelope("origin", []byte(fmt.Sprintf("data-%d-%d", n, j)))
				env.ID = fmt.Sprintf("env-%d-%d", n, j)
				if err := shelf.Push(env); err != nil {
					t.Errorf("push %d-%d: %v", n, j, err)
				}
			}
		}(i)
	}
	wg.Wait()

	items := shelf.Peek()
	if len(items) != 50 {
		t.Errorf("expected 50 envelopes, got %d", len(items))
	}
}

func TestDoltShelfConcurrentPushPull(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "racedb")
	db, err := store.Open(dir)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer db.Close()
	if err := db.Migrate(); err != nil {
		t.Fatalf("migrate: %v", err)
	}

	dp := NewDoltDepo(db.DB, "race")
	shelf := dp.Shelf("work")

	for i := range 20 {
		env := artifact.NewEnvelope("origin", []byte("data"))
		env.ID = fmt.Sprintf("env-%d", i)
		_ = shelf.Push(env)
	}

	var wg sync.WaitGroup
	var mu sync.Mutex
	pulled := 0
	errors := 0

	for i := range 5 {
		wg.Add(1)
		go func(n int) {
			defer wg.Done()
			for range 10 {
				_, err := shelf.Pull(fmt.Sprintf("agent-%d", n))
				mu.Lock()
				if err == nil {
					pulled++
				} else {
					errors++
				}
				mu.Unlock()
			}
		}(i)
	}
	wg.Wait()

	if pulled != 20 {
		t.Errorf("expected 20 pulled, got %d (errors: %d)", pulled, errors)
	}
}
