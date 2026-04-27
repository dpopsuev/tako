package artifact

// Wire is a typed, codec-agnostic message on the Terminal.
type Wire struct {
	Kind    string
	Channel string
	Payload []byte
}
