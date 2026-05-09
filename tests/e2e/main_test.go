package e2e

import (
	"fmt"
	"os"
	"os/exec"
	"testing"
)

func TestMain(m *testing.M) {
	out, err := exec.Command("go", "build", "github.com/dpopsuev/tako/cmd/tako").CombinedOutput()
	if err != nil {
		fmt.Fprintf(os.Stderr, "pre-condition failed: tako binary does not compile\n%s", out)
		os.Exit(1)
	}
	os.Exit(m.Run())
}
