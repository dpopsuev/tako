package dispatch

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

const statusComplete = "complete"

// FinalizeSignals walks the calibration directory and sets every signal.json
// to status statusComplete.
func FinalizeSignals(dir string) {
	count := 0
	_ = filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}
		if info.IsDir() || info.Name() != "signal.json" {
			return nil
		}

		data, err := os.ReadFile(path)
		if err != nil {
			return nil
		}

		var sig SignalFile
		if err := json.Unmarshal(data, &sig); err != nil {
			return nil
		}

		if sig.Status == statusComplete {
			return nil
		}

		sig.Status = statusComplete
		sig.Timestamp = time.Now().UTC().Format(time.RFC3339)
		if err := WriteSignal(path, &sig); err != nil {
			fmt.Fprintf(os.Stderr, "[lifecycle] finalize %s: %v\n", path, err)
			return nil
		}
		count++
		return nil
	})

	if count > 0 {
		fmt.Printf("[lifecycle] finalized %d signal(s) to status=complete\n", count)
	}
}
