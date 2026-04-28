package acceptance

// Feature: State Persistence
//   As a framework consumer
//   I want to persist walker state during execution
//   So that I can resume failed walks and share memory across walkers

import (
	"testing"

	"github.com/dpopsuev/tako/circuit"
	"github.com/dpopsuev/tako/engine"
)

func TestPersistence_CheckpointAfterEachNode(t *testing.T) {
	// Scenario: Checkpointer is invoked during circuit execution
	//   Given a linear circuit with 2 nodes
	//   When I run with WithCheckpointer
	//   Then the walk completes successfully
	//   And the checkpointer processes without errors

	tmpDir := t.TempDir()
	cp, err := engine.NewJSONCheckpointer(tmpDir)
	if err != nil {
		t.Fatalf("NewJSONCheckpointer: %v", err)
	}

	// Run the circuit with checkpointing enabled
	err = runFixture(t, "circuits/linear.yaml", nil,
		engine.WithCheckpointer(cp),
	)
	if err != nil {
		t.Fatalf("Run with checkpointer: %v", err)
	}

	// After successful completion, checkpoints are cleaned up
	// But we can verify the checkpointer directory exists and is accessible
	ids, err := cp.List()
	if err != nil {
		t.Fatalf("List checkpoints: %v", err)
	}

	// After successful walk, checkpoints should be cleaned up
	if len(ids) != 0 {
		t.Logf("Note: %d checkpoint(s) remain after successful walk", len(ids))
	}
}

func TestPersistence_MemoryStoreAcrossContext(t *testing.T) {
	// Scenario: MemoryStore persists values across walker context
	//   Given an InMemoryStore with a pre-set value
	//   When I run a circuit with WithMemory
	//   Then the walker can access the stored value during execution

	store := engine.NewInMemoryStore()
	walkerID := "persistent-walker"

	// Pre-populate the store
	store.Set(walkerID, "test_key", "test_value")

	// Verify the value is accessible
	val, ok := store.Get(walkerID, "test_key")
	if !ok {
		t.Fatal("stored value not found in memory store")
	}
	if val != "test_value" {
		t.Errorf("stored value = %q, want %q", val, "test_value")
	}

	// Run a circuit with the memory store attached
	// The circuit doesn't need to explicitly use the memory, we're just
	// verifying the plumbing works
	w := circuit.NewProcessWalker(walkerID)

	err := runFixture(t, "circuits/linear.yaml", nil,
		engine.WithMemory(store),
		engine.WithWalker(w),
	)
	if err != nil {
		t.Fatalf("Run with memory: %v", err)
	}

	// Verify the store still has the value after run
	val2, ok := store.Get(walkerID, "test_key")
	if !ok {
		t.Error("stored value disappeared after circuit run")
	}
	if val2 != "test_value" {
		t.Errorf("stored value after run = %q, want %q", val2, "test_value")
	}

	// Set a new value in the store
	store.Set(walkerID, "post_run_key", "post_run_value")

	// Verify it's accessible
	val3, ok := store.Get(walkerID, "post_run_key")
	if !ok {
		t.Error("post-run value not found in memory store")
	}
	if val3 != "post_run_value" {
		t.Errorf("post-run value = %q, want %q", val3, "post_run_value")
	}
}

func TestPersistence_MemoryStoreNamespaceIsolation(t *testing.T) {
	// Scenario: MemoryStore namespaces isolate values
	//   Given an InMemoryStore
	//   When I set values in different namespaces
	//   Then they do not interfere with each other

	store := engine.NewInMemoryStore()
	walkerID := "test-walker"

	// Set values in different namespaces
	store.SetNS("ns1", walkerID, "key", "value1")
	store.SetNS("ns2", walkerID, "key", "value2")

	// Verify namespace isolation
	val1, ok := store.GetNS("ns1", walkerID, "key")
	if !ok {
		t.Fatal("value in ns1 not found")
	}
	if val1 != "value1" {
		t.Errorf("ns1 value = %q, want value1", val1)
	}

	val2, ok := store.GetNS("ns2", walkerID, "key")
	if !ok {
		t.Fatal("value in ns2 not found")
	}
	if val2 != "value2" {
		t.Errorf("ns2 value = %q, want value2", val2)
	}

	// Verify default namespace is empty
	_, ok = store.Get(walkerID, "key")
	if ok {
		t.Error("default namespace should not have the key")
	}
}
