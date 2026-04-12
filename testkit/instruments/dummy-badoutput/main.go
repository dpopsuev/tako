// dummy-badoutput: returns JSON missing the required "status" field.
// Proves runtime output schema validation rejects non-compliant output.
package main

import (
	"encoding/json"
	"fmt"
	"os"
)

func main() {
	// Consume stdin.
	var input map[string]any
	json.NewDecoder(os.Stdin).Decode(&input)

	// Return valid JSON but missing the "status" field that the schema requires.
	out := map[string]string{"wrong": "field"}
	data, _ := json.Marshal(out)
	fmt.Println(string(data))
}
