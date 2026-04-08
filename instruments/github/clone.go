package github

import (
	"context"
	"fmt"
	"os/exec"
	"time"
)

const cloneTimeout = 60 * time.Second

// shallowClone performs a depth-1 clone of the given branch into dest.
// When a token is provided, it uses GIT_ASKPASS instead of embedding the
// token in the URL (which would be visible in process listings).
func shallowClone(ctx context.Context, url, branch, dest, token string) error {
	ctx, cancel := context.WithTimeout(ctx, cloneTimeout)
	defer cancel()

	args := []string{"clone", "--depth=1", "--single-branch"}
	if branch != "" {
		args = append(args, "--branch", branch)
	}
	args = append(args, url, dest)

	cmd := exec.CommandContext(ctx, "git", args...)
	if token != "" {
		// Use GIT_ASKPASS to provide the token without embedding in URL.
		// The "echo" script returns the token when git asks for a password.
		cmd.Env = append(cmd.Environ(),
			"GIT_ASKPASS=echo",
			"GIT_USERNAME=x-access-token",
			"GIT_PASSWORD="+token,
			"GIT_TERMINAL_PROMPT=0",
		)
	}
	output, err := cmd.CombinedOutput()
	if err != nil {
		// Sanitize: never include token or full URL in error message.
		return fmt.Errorf("git clone %s@%s: %w\n%s", sanitizeURL(url), branch, err, output)
	}
	return nil
}

func sanitizeURL(url string) string {
	// Strip embedded credentials from URL for safe logging.
	if i := len("https://"); len(url) > i {
		if at := indexOf(url[i:], '@'); at >= 0 {
			return url[:i] + "***@" + url[i+at+1:]
		}
	}
	return url
}

func indexOf(s string, ch byte) int {
	for i := 0; i < len(s); i++ {
		if s[i] == ch {
			return i
		}
	}
	return -1
}
