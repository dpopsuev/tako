// dummy-upper: reads JSON from stdin, returns {"upper": "UPPERCASED"}
// Compiled Go binary — proves language-agnostic instrument dispatch.
//
// Usage: dummy-upper [subcommand]
// Subcommand is ignored — the binary always uppercases stdin JSON.
package main

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
)

func main() {
	var input map[string]any
	if err := json.NewDecoder(os.Stdin).Decode(&input); err != nil {
		input = map[string]any{}
	}

	text, _ := input["text"].(string)
	if text == "" {
		text = "default"
	}

	out := map[string]string{"upper": strings.ToUpper(text)}
	data, _ := json.Marshal(out)
	fmt.Println(string(data))
}
