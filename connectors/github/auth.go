package github

import (
	"fmt"
	"os"
	"strings"
)

const (
	defaultTokenFile       = ".github-token"
	tokenEnvVar            = "GITHUB_TOKEN"
	privateRepoTokensEnvVar = "GITHUB_PRIVATE_REPO_TOKENS"
)

// ResolveToken finds a GitHub token using the following precedence:
//  1. $GITHUB_TOKEN environment variable
//  2. File at the given path (first line, trimmed)
//  3. File at the default path (.github-token)
//
// Returns empty string with no error if no token is found (public repos only).
func ResolveToken(tokenFilePath string) (string, error) {
	if tok := os.Getenv(tokenEnvVar); tok != "" {
		return strings.TrimSpace(tok), nil
	}

	paths := []string{tokenFilePath, defaultTokenFile}
	for _, p := range paths {
		if p == "" {
			continue
		}
		data, err := os.ReadFile(p)
		if os.IsNotExist(err) {
			continue
		}
		if err != nil {
			return "", fmt.Errorf("read token file %s: %w", p, err)
		}
		line := strings.TrimSpace(strings.SplitN(string(data), "\n", 2)[0])
		if line != "" {
			return line, nil
		}
	}
	return "", nil
}

// ResolvePrivateRepoTokens parses $GITHUB_PRIVATE_REPO_TOKENS into a
// map of org (or org/repo) to token. Format: "org1=token1,org2=token2".
// Returns nil if the env var is empty.
func ResolvePrivateRepoTokens() map[string]string {
	raw := os.Getenv(privateRepoTokensEnvVar)
	if raw == "" {
		return nil
	}
	tokens := make(map[string]string)
	for _, entry := range strings.Split(raw, ",") {
		entry = strings.TrimSpace(entry)
		if k, v, ok := strings.Cut(entry, "="); ok && k != "" && v != "" {
			tokens[strings.TrimSpace(k)] = strings.TrimSpace(v)
		}
	}
	if len(tokens) == 0 {
		return nil
	}
	return tokens
}

// TokenForOrg returns the most specific token for the given org/repo pair.
// Checks org/repo first, then org, then falls back to the default token.
func TokenForOrg(privateTokens map[string]string, org, repo, defaultToken string) string {
	if privateTokens != nil {
		if tok, ok := privateTokens[org+"/"+repo]; ok {
			return tok
		}
		if tok, ok := privateTokens[org]; ok {
			return tok
		}
	}
	return defaultToken
}

// cloneURL builds the HTTPS clone URL, optionally embedding the token
// for private repo access.
func cloneURL(org, repo, token string) string {
	if token != "" {
		return fmt.Sprintf("https://%s@github.com/%s/%s.git", token, org, repo)
	}
	return fmt.Sprintf("https://github.com/%s/%s.git", org, repo)
}
