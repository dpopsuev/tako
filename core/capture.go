package core

// Category: Processing & Support

// ArtifactCapture provides access to artifacts captured during a walk.
// Obtain one via NewCapture() and use the returned WalkObserver during the walk.
type ArtifactCapture interface {
	ArtifactAt(node string) (Artifact, bool)
	Artifacts() map[string]Artifact
}
