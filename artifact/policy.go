package artifact

import "errors"

var (
	ErrHashMismatch = errors.New("artifact: envelope hash verification failed")
	ErrPolicyReject = errors.New("artifact: policy rejected envelope")
)

// Policy enforces label rules on Shelf operations.
// Contract stamps labels. Policy protects them. Shelf stores.
type Policy interface {
	OnPush(shelf string, envelope Envelope) error
	OnPull(shelf string, envelope Envelope, agentID string) error
}

// AlwaysAllowPolicy is a stub policy that accepts everything.
type AlwaysAllowPolicy struct{}

var _ Policy = AlwaysAllowPolicy{}

func (AlwaysAllowPolicy) OnPush(_ string, _ Envelope) error           { return nil }
func (AlwaysAllowPolicy) OnPull(_ string, _ Envelope, _ string) error { return nil }

// VerifyHashPolicy rejects envelopes with broken hash seals.
type VerifyHashPolicy struct{}

var _ Policy = VerifyHashPolicy{}

func (VerifyHashPolicy) OnPush(_ string, e Envelope) error {
	if !e.Verify() {
		return ErrHashMismatch
	}
	return nil
}

func (VerifyHashPolicy) OnPull(_ string, _ Envelope, _ string) error { return nil }
