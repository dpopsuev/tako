package finding

// Category: Processing & Support

import "github.com/dpopsuev/tako/circuit"

// VetoArtifact wraps an artifact and overrides Confidence to 0.
// Used by the hookingWalker when a VetoHook returns ErrFindingVeto.
type VetoArtifact struct {
	Inner circuit.Artifact
}

func (v *VetoArtifact) Type() string        { return v.Inner.Type() }
func (v *VetoArtifact) Confidence() float64 { return 0 }
func (v *VetoArtifact) Raw() any            { return v.Inner.Raw() }
