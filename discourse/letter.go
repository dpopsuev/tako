package discourse

import "time"

// Letter is the universal message primitive.
type Letter struct {
	From      string
	To        string
	Subject   string
	Body      string
	CreatedAt time.Time
}

// Thread is an ordered sequence of Letters on one topic.
type Thread struct {
	ID      string
	Letters []Letter
}

// Topic groups Threads on a Board.
type Topic struct {
	Name    string
	Threads []Thread
}
