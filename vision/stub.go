package vision

// StubStream is a noop — subscribe returns a closed channel.
type StubStream struct{}

var _ Stream = StubStream{}

func (StubStream) Subscribe(_ string) <-chan Frame {
	ch := make(chan Frame)
	close(ch)
	return ch
}

func (StubStream) Unsubscribe(_ string) {}
