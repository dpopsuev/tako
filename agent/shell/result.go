package shell

// Result is the structured output of an instrument execution.
type Result struct {
	Content []Content `json:"content"`
	IsError bool      `json:"isError,omitempty"`
}

func (r Result) Text() []byte {
	var buf []byte
	for _, c := range r.Content {
		if c.Type == "text" {
			buf = append(buf, c.Text...)
		}
	}
	return buf
}

func TextResult(text string) Result {
	return Result{Content: []Content{TextContent(text)}}
}

func ErrorResult(text string) Result {
	return Result{
		Content: []Content{ErrorContent(text)},
		IsError: true,
	}
}
