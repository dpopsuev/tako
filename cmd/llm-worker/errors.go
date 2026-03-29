package main

import "errors"

var (
	// ErrANTHROPICAPIKEYRequiredForClaudeProvider is returned for: ANTHROPIC_API_KEY required for claude provider
	ErrANTHROPICAPIKEYRequiredForClaudeProvider = errors.New("ANTHROPIC_API_KEY required for claude provider")

	// ErrGEMINIAPIKEYRequiredForGeminiProvider is returned for: GEMINI_API_KEY required for gemini provider
	ErrGEMINIAPIKEYRequiredForGeminiProvider = errors.New("GEMINI_API_KEY required for gemini provider")

	// ErrOPENAIAPIKEYRequiredForOpenaiProvider is returned for: OPENAI_API_KEY required for openai provider
	ErrOPENAIAPIKEYRequiredForOpenaiProvider = errors.New("OPENAI_API_KEY required for openai provider")

	// ErrUnknownProvider is returned for: unknown provider
	ErrUnknownProvider = errors.New("unknown provider")

	// ErrOllamaStatus is returned for: ollama: status
	ErrOllamaStatus = errors.New("ollama: status")

	// ErrAnthropicStatus is returned for: anthropic: status
	ErrAnthropicStatus = errors.New("anthropic: status")

	// ErrGeminiStatus is returned for: gemini: status
	ErrGeminiStatus = errors.New("gemini: status")

	// ErrGeminiEmptyResponse is returned for: gemini: empty response
	ErrGeminiEmptyResponse = errors.New("gemini: empty response")

	// ErrOpenaiStatus is returned for: openai: status
	ErrOpenaiStatus = errors.New("openai: status")

	// ErrOpenaiEmptyResponse is returned for: openai: empty response
	ErrOpenaiEmptyResponse = errors.New("openai: empty response")

	// ErrCircuitStartError is returned for: circuit/start error
	ErrCircuitStartError = errors.New("circuit/start error")
)
