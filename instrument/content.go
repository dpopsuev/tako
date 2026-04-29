package instrument

// Content is one part of an instrument Result.
type Content struct {
	Type     string `json:"type"`
	Text     string `json:"text,omitempty"`
	Data     []byte `json:"data,omitempty"`
	MimeType string `json:"mimeType,omitempty"`
}

func TextContent(text string) Content {
	return Content{Type: "text", Text: text}
}

func ErrorContent(text string) Content {
	return Content{Type: "text", Text: text}
}
