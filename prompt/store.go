package prompt

import "errors"

// Sentinel errors for prompt operations.
var (
	ErrNotFound       = errors.New("prompt not found")
	ErrVersionMissing = errors.New("version not found in history")
	ErrNameRequired   = errors.New("prompt name is required")
	ErrContentEmpty   = errors.New("prompt content is empty")
	ErrReadOnly       = errors.New("file store is read-only")
	ErrAlreadyExists  = errors.New("prompt already exists")
)

// Store is the interface for prompt storage and versioning.
// FilePromptStore implements read-only access from fs.FS.
// LivePromptStore implements full CRUD with version history.
type Store interface {
	// Get returns a prompt by name.
	Get(name string) (*Prompt, error)

	// List returns all prompts.
	List() ([]*Prompt, error)

	// Update replaces prompt content and increments the version.
	// Returns ErrNotFound if the prompt does not exist.
	Update(name, content string) (*Prompt, error)

	// Create adds a new prompt. Returns an error if it already exists.
	Create(name, step, content string) (*Prompt, error)

	// Rollback reverts a prompt to a previous version.
	Rollback(name string, version int) (*Prompt, error)
}
