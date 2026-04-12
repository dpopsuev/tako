package assertions

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/dpopsuev/origami/engine"
)

// AssertParked verifies that a pending approval item exists for the given node.
// On failure, dumps the full store contents for debugging.
func AssertParked(tb testing.TB, store engine.ApprovalStore, nodeName string) engine.ApprovalItem {
	tb.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	items, err := store.List(ctx, engine.ApprovalPending)
	if err != nil {
		tb.Fatalf("AssertParked: List failed: %v", err)
	}

	for _, item := range items {
		if item.NodeName == nodeName {
			return item
		}
	}

	// Failure — dump store for debugging.
	tb.Fatalf("AssertParked: no pending item for node %q\n%s", nodeName, dumpStore(ctx, store))
	return engine.ApprovalItem{} // unreachable
}

// AssertApproved verifies that an approval item has been approved.
// On failure, dumps the item's current state.
func AssertApproved(tb testing.TB, store engine.ApprovalStore, itemID string) {
	tb.Helper()
	ctx := context.Background()

	item, err := store.Get(ctx, itemID)
	if err != nil {
		tb.Fatalf("AssertApproved: Get(%q) failed: %v", itemID, err)
	}
	if item.Status != engine.ApprovalApproved {
		tb.Fatalf("AssertApproved: item %q status = %q, want %q\n%s",
			itemID, item.Status, engine.ApprovalApproved, dumpItem(item))
	}
}

// AssertRejected verifies that an approval item has been rejected.
func AssertRejected(tb testing.TB, store engine.ApprovalStore, itemID string) {
	tb.Helper()
	ctx := context.Background()

	item, err := store.Get(ctx, itemID)
	if err != nil {
		tb.Fatalf("AssertRejected: Get(%q) failed: %v", itemID, err)
	}
	if item.Status != engine.ApprovalRejected {
		tb.Fatalf("AssertRejected: item %q status = %q, want %q\n%s",
			itemID, item.Status, engine.ApprovalRejected, dumpItem(item))
	}
}

// AssertNoPending verifies that the store has no pending items.
func AssertNoPending(tb testing.TB, store engine.ApprovalStore) {
	tb.Helper()
	ctx := context.Background()

	items, err := store.List(ctx, engine.ApprovalPending)
	if err != nil {
		tb.Fatalf("AssertNoPending: List failed: %v", err)
	}
	if len(items) != 0 {
		tb.Fatalf("AssertNoPending: %d pending items remain\n%s", len(items), dumpStore(ctx, store))
	}
}

// dumpStore returns a formatted summary of all store items for debugging.
func dumpStore(ctx context.Context, store engine.ApprovalStore) string {
	var all []engine.ApprovalItem
	for _, status := range []engine.ApprovalStatus{engine.ApprovalPending, engine.ApprovalApproved, engine.ApprovalRejected} {
		items, _ := store.List(ctx, status)
		all = append(all, items...)
	}
	if len(all) == 0 {
		return "  store: (empty)"
	}
	var b strings.Builder
	for i, item := range all {
		fmt.Fprintf(&b, "  [%d] id=%s node=%s status=%s parked=%s\n",
			i, item.ID, item.NodeName, item.Status, item.ParkedAt.Format(time.RFC3339))
	}
	return b.String()
}

// dumpItem returns a JSON representation of an item for debugging.
func dumpItem(item *engine.ApprovalItem) string {
	data, _ := json.MarshalIndent(item, "  ", "  ")
	return "  " + string(data)
}
