package main

import (
	"fmt"
	"os"
)

func main() {
	os.Exit(0)
}

// unused import "fmt" is the planted lint issue.
// The fix patch removes it.
var _ = fmt.Sprintf
